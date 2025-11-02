package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/aminkamal/lol/internal/cruncher"
	"github.com/aminkamal/lol/internal/scraper"
	"github.com/aminkamal/lol/pkg/logger"
	"github.com/aminkamal/lol/pkg/riot"
	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagent"
	brt "github.com/aws/aws-sdk-go-v2/service/bedrockagent/types"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	// apparently haiku is faster than sonnet but only by 2-3s for a simple question
	// Haiku is so bad at generating SQL queries, don't use it.
	modelInstanceProfile = "global.anthropic.claude-sonnet-4-5-20250929-v1:0"

	s3Bucket               = "<BUCKET_NAME>"
	awsGluePrefix          = "table"
	kbMarkdownId           = "<ID_HERE>"
	kbMarkdownDescription  = "Match-specific summaries. You can find relevant information using the string match id, match date, match outcome (Won/Lost) or the composite key on riotidgamename and riotidtagline."
	kbMarkdownDataSourceId = "<ID_HERE>"
	kbRedshiftId           = "<ID_HERE>"
	kbRedshiftDescription  = `This knowledge base contains match summaries for many players. Use it when asked for queries to get interesting statistics or when generating plots and graphs.`
	routerInstructions     = `
	You are a classification agent, your job is to route the user's question to the correct agent.
	You can also get the user's matches, but only if the user specifically says "get my match history for 2024" verbatim.
	`
	agent47Instructions = `
		You are a helpful assistant that can answer questions about the user's League of Legends matches for the year 2024.
		You are humorous with a gaming personality, don't act too formal. Be fun.

		You have access to a knowledge base that contains match results for the user and others for that year.
		- You MUST use the Riot Id Game Name (riotidgamename) and Tag Line (riotidtagline) when generating queries to get data relevant to the user.
		- For example, Sneaky#NA69 has Sneaky as the game name and NA69 as the tag line.
		- When asked for a graph or a plot. ONLY return the generated HTML (using chart.js) inside srcdoc property of an iframe, do NOT wrap the html in anything and start your response with <iframe>.
		- If the year isn't specified, use 2024.

		Additional instructions for you:
		- The user may ask you to roast them, take a jab at their username (game name and/or tag line) and performance results.
		- The user may ask you to toast them, find something nice to say about their username and/or performance results.
		- When emphasizing text, don't use markdown, use HTML <b> instead.

		And finally:
		- When referencing gamestarttimestamp column, don't forget to cast it as a date: CAST(gamestarttimestamp AS DATE)
		`
	librarianInstructions = `
		You are a helpful assistant that can answer questions about the user's League of Legends matches for the year 2024.
		You are humorous with a gaming personality, don't act too formal. Be fun.

		You can coach players to improve their play, you will offer constructive feedback on how they can reach a high elo (e.g. Silver 2).
		As a coach, you have access to another knowledge base that contains match summaries, access the matches by riotidgamename and riotidtagline, match id or the calendar month the match was played in.
		If the user doesn't specify which match id or date to have coaching on. Choose any match date randomly (but still specific to that riotidgamename and riotidtagline).

		Additional instructions for you:
		- Always show the match id and date
		- Always show the statistic or data when suggesting improvements
		- NEVER offer generic advice. Suggestions must be backed by a reference in the match summary.
		- The user may ask you to roast them, take a jab at their username (game name and/or tag line) and performance results.
		- The user may ask you to toast them, find something nice to say about their username and/or performance results.
		- When emphasizing text, don't use markdown, use HTML <b> instead.

		And finally:
		- When referencing gamestarttimestamp column, don't forget to cast it as a date: CAST(gamestarttimestamp AS DATE)
		`
	kbIngestionMaxRetries = 5
	kbIngestionBatchSize  = 10
)

type ResponseStreamHandler = func(context.Context, <-chan types.InlineAgentResponseStream)

type S3UploadJob struct {
	FilePath string
	Key      string
}

type service struct {
	cfg                aws.Config
	riotClient         *riot.Client
	bedrockClient      *bedrockagentruntime.Client
	bedrockAgentClient *bedrockagent.Client
	s3Client           *s3.Client
	athenaClient       *athena.Client

	sessionIdChanMap map[string]chan string
}

func New(cfg aws.Config, apiKey string) *service {
	return &service{
		cfg:                cfg,
		riotClient:         riot.NewClient(apiKey),
		bedrockClient:      bedrockagentruntime.NewFromConfig(cfg),
		bedrockAgentClient: bedrockagent.NewFromConfig(cfg),
		s3Client:           s3.NewFromConfig(cfg),
		athenaClient:       athena.NewFromConfig(cfg),

		sessionIdChanMap: map[string]chan string{},
	}
}

func (s *service) HandleLanding(tpl *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		message := "Ensure you use the correct URL. Please check the submission for the correct one. HINT: You're missing a /**something**"
		w.Write([]byte(message))
	}
}

func (s *service) HandleIndex(tpl *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl.ExecuteTemplate(w, "index.html", nil)
	}
}

func (s *service) HandleSession(w http.ResponseWriter, r *http.Request) {
	var request InitialSessionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&request)
	if strings.TrimSpace(request.GameName) == "" ||
		strings.TrimSpace(request.TagLine) == "" {
		w.WriteHeader(400)
		return
	}

	sessionId := uuid.NewString()
	response := InitialSessionResponse{
		SessionId: sessionId,
	}

	s.sessionIdChanMap[sessionId] = make(chan string)

	jason, _ := json.Marshal(response)
	w.Write(jason)
}

func (s *service) EventHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Create a flusher to send data immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	qs := r.URL.Query()
	// blah blah security, use cookies to pass the session_id
	sessionId := qs.Get("session_id")
	if sessionId == "" {
		fmt.Fprintf(w, "event: close\n")
		fmt.Fprintf(w, "data: close\n\n")
		flusher.Flush()
		return
	}

	// Send initial connection message
	fmt.Fprintf(w, "event: system\n")
	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	// Create a ticker for periodic updates
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Create a channel to detect client disconnect
	clientGone := r.Context().Done()

	for {
		select {
		case <-clientGone:
			close(s.sessionIdChanMap[sessionId])
			delete(s.sessionIdChanMap, sessionId)

			logger.Debug("Client disconnected")
			return

		case <-ticker.C:
			fmt.Fprintf(w, "event: system\n")
			fmt.Fprintf(w, "data: ping\n\n")

			flusher.Flush()

		case message := <-s.sessionIdChanMap[sessionId]:
			// can use a struct for different message type s
			// and assert on its type and get the contents but this'll do for now
			if message == "TOGGLE_LOADING" {
				fmt.Fprintf(w, "event: toggle_loading\n")
				fmt.Fprintf(w, "data: toggle_loading\n\n")
				flusher.Flush()
				continue
			}

			// hackity hack
			if strings.HasPrefix(message, "FEATURES||") {
				jason := message[len("FEATURES||"):]
				type feature struct {
					Icon        string `json:"icon"`
					Title       string `json:"title"`
					Description string `json:"description"`
				}

				var features []feature
				err := json.Unmarshal([]byte(jason), &features)
				if err != nil {
					logger.Error("failed to unmarshal features: %s in %s", err.Error(), jason)
					continue
				}

				for _, f := range features {
					fmt.Fprintf(w, "event: feature\n")
					fJson, _ := json.Marshal(f)
					fmt.Fprintf(w, "data: %s\n\n", fJson)
					flusher.Flush()
				}
				continue
			}

			fmt.Fprintf(w, "event: message\n")
			sEnc := base64.StdEncoding.EncodeToString([]byte(message))

			fmt.Fprintf(w, "data: %s\n\n", kaisaFix(replaceSpaces(sEnc)))

			flusher.Flush()
		}
	}
}

func (s *service) ChatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var request ChatRequest
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&request)
	if strings.TrimSpace(request.SessionId) == "" ||
		strings.TrimSpace(request.Message) == "" {
		w.WriteHeader(400)
		return
	}

	if _, ok := s.sessionIdChanMap[request.SessionId]; !ok {
		w.WriteHeader(400)
		return
	}

	logger.Debug("[%s] User: %s", request.SessionId, request.Message)

	err := s.invoke(ctx, request.SessionId, request.Message, s.responseStreamHandler(request.SessionId, false), nil)

	if err != nil {
		logger.Error("%s", err.Error())
	}
}

func (s *service) invoke(
	ctx context.Context,
	sessionId string,
	inputText string,
	fn ResponseStreamHandler,
	inlineSessionState *types.InlineSessionState,
) error {
	iai := &bedrockagentruntime.InvokeInlineAgentInput{
		// Agent configuration
		FoundationModel:         aws.String(modelInstanceProfile),
		Instruction:             aws.String(routerInstructions),
		AgentCollaboration:      types.AgentCollaborationSupervisorRouter,
		IdleSessionTTLInSeconds: aws.Int32(3600),
		ActionGroups: []types.AgentActionGroup{
			{
				ActionGroupName: aws.String("AskRiotForMatchData"),
				ActionGroupExecutor: &types.ActionGroupExecutorMemberCustomControl{
					Value: types.CustomControlMethodReturnControl,
				},
				ApiSchema: &types.APISchemaMemberPayload{
					Value: *aws.String(OAPI),
				},
			},
		},
		Collaborators: []types.Collaborator{
			{
				AgentName:       aws.String("Agent47"),
				Instruction:     aws.String(agent47Instructions),
				FoundationModel: aws.String(modelInstanceProfile),
				KnowledgeBases: []types.KnowledgeBase{
					{
						KnowledgeBaseId: aws.String(kbRedshiftId),
						Description:     aws.String(kbRedshiftDescription),
					},
				},
			},
			{
				AgentName:       aws.String("AgentLibrarian"),
				Instruction:     aws.String(librarianInstructions),
				FoundationModel: aws.String(modelInstanceProfile),
				KnowledgeBases: []types.KnowledgeBase{
					{
						KnowledgeBaseId: aws.String(kbMarkdownId),
						Description:     aws.String(kbMarkdownDescription),
						RetrievalConfiguration: &types.KnowledgeBaseRetrievalConfiguration{
							VectorSearchConfiguration: &types.KnowledgeBaseVectorSearchConfiguration{
								// NumberOfResults:    aws.Int32(3),
								OverrideSearchType: types.SearchTypeSemantic,
								ImplicitFilterConfiguration: &types.ImplicitFilterConfiguration{
									ModelArn: aws.String(modelInstanceProfile),
									MetadataAttributes: []types.MetadataAttributeSchema{
										{
											Key:         aws.String("riotidgamename"),
											Description: aws.String("Riot Id game name of the user"),
											Type:        types.AttributeTypeString,
										},
										{
											Key:         aws.String("riotidtagline"),
											Description: aws.String("Riot Id tag line of the user"),
											Type:        types.AttributeTypeString,
										},
										{
											Key:         aws.String("match_result"),
											Description: aws.String("Result of that match; Won or Lost"),
											Type:        types.AttributeTypeString,
										},
										{
											Key:         aws.String("match_month"),
											Description: aws.String("the month name the match was played in"),
											Type:        types.AttributeTypeString,
										},
										{
											Key:         aws.String("match_id"),
											Description: aws.String("Unique Id of the match"),
											Type:        types.AttributeTypeString,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		CollaboratorConfigurations: []types.CollaboratorConfiguration{
			{
				CollaboratorName: aws.String("Agent47"),
				CollaboratorInstruction: aws.String(`
				You are an expert at drawing plots and answering questions that relate to aggregates (average, sum, min, max) and questions about the most and the least.`),
				RelayConversationHistory: types.RelayConversationHistoryToCollaborator,
			},
			{
				CollaboratorName: aws.String("AgentLibrarian"),
				CollaboratorInstruction: aws.String(`
				You are an expert at answering coaching questions and giving tips on how to improve their League of Legends play.
				You have access to summaries of player matches for the year 2024.
				`),
				RelayConversationHistory: types.RelayConversationHistoryToCollaborator,
			},
		},
		SessionId:          &sessionId,
		InputText:          &inputText,
		InlineSessionState: inlineSessionState,
		EnableTrace:        aws.Bool(true),
	}

	resp, err := s.bedrockClient.InvokeInlineAgent(ctx, iai)

	if err != nil {
		return err
	}

	eventStream := resp.GetStream()
	defer eventStream.Close()

	// Process events
	fn(ctx, eventStream.Events())

	if err := eventStream.Err(); err != nil {
		logger.Error("%s", err.Error())
		return err
	}

	return err
}

func (s *service) responseStreamHandler(sessionId string, isFeaturesSummary bool) func(ctx context.Context, c <-chan types.InlineAgentResponseStream) {
	return func(ctx context.Context, c <-chan types.InlineAgentResponseStream) {
		var invocationID *string

		for event := range c {
			switch e := event.(type) {
			case *types.InlineAgentResponseStreamMemberChunk:
				// Agent's text response
				resp := string(e.Value.Bytes)
				logger.Debug("[%s] Agent: %s", sessionId, resp)

				if isFeaturesSummary {
					s.sessionIdChanMap[sessionId] <- "FEATURES||" + strings.ReplaceAll(strings.ReplaceAll(resp, "```json", ""), "```", "")
				} else {
					s.sessionIdChanMap[sessionId] <- resp
				}

			case *types.InlineAgentResponseStreamMemberReturnControl:
				// Agent is returning control - it wants us to execute the tool!
				logger.Debug("[%s] RETURN_CONTROL Event", sessionId)
				invocationID = e.Value.InvocationId

				logger.Debug("[%s] Invocation ID: %s", sessionId, *invocationID)

				// Process each tool invocation request
				for _, input := range e.Value.InvocationInputs {
					switch inv := input.(type) {
					case *types.InvocationInputMemberMemberApiInvocationInput:
						actionGroup := *inv.Value.ActionGroup
						apiPath := *inv.Value.ApiPath

						logger.Debug("[%s] Action Group: %s", sessionId, actionGroup)
						logger.Debug("[%s] API Path: %s", sessionId, apiPath)

						// Extract parameters
						params := make(map[string]string)
						for _, p := range inv.Value.RequestBody.Content {
							for _, prop := range p.Properties {
								params[*prop.Name] = *prop.Value
							}
						}

						logger.Debug("[%s] Parameters: %+v", sessionId, params)

						s.sessionIdChanMap[sessionId] <- "Collecting your match history, this might take a while."
						s.sessionIdChanMap[sessionId] <- "TOGGLE_LOADING"

						// Execute the tool locally
						result, err := s.InvokeTool(ctx, sessionId, params["gamename"], params["tagLine"])
						if err != nil {
							logger.Debug("[%s] Tool failed: %s", sessionId, err.Error())
							return
						}

						preface := fmt.Sprintf(`
						Remember that the user's Riot Id game name (riotidgamename) is %s, and Riot Id tag line is %s.

						Use the following summary to generate the player's "Rift Rewind 2024" results, this report contains:
						- Overall number of matches, and hours spent on the rift
						- Win/Loss ratio
						- Overall performance in matches (think gold per minute, KDA)
						- Champion-specific performance

						Afterwards, offer additional assistance to the player such as:
						- In-depth statistics, such as the total number of gold hoarded when playing as a certain champion or total time played as that champion...etc
						- Other players the user won against or lost against the most
						- Best or worst months
						- Offer plotting those results						
						`, params["gamename"], params["tagLine"])

						// Send results back to agent
						s.sendToolResultsBack(ctx, sessionId, invocationID, actionGroup, apiPath, preface+"\n\n"+string(result))
					}
				}

			case *types.InlineAgentResponseStreamMemberTrace:
				// Debug traces (optional)
				// trace, _ := json.MarshalIndent(e.Value, "", "  ")
				// logger.Debug("%s", trace)

				collabName := ""
				if e.Value.CollaboratorName != nil {
					collabName = *e.Value.CollaboratorName
				}
				logger.Debug("[%s] Collaborator: %s", sessionId, collabName)

			default:
				logger.Debug("[%s] Other event: %T", sessionId, e)
			}
		}
	}
}

// sendToolResultsBack sends the tool execution results back to the agent
func (s *service) sendToolResultsBack(
	ctx context.Context,
	sessionId string,
	invocationId *string,
	actionGroup, apiPath, result string,
) {
	logger.Debug("[%s] Sending Results Back to Agent: %s", sessionId, result)

	iss := &types.InlineSessionState{
		InvocationId: invocationId,
		ReturnControlInvocationResults: []types.InvocationResultMember{
			&types.InvocationResultMemberMemberApiResult{
				Value: types.ApiResult{
					ActionGroup:    aws.String(actionGroup),
					ApiPath:        aws.String(apiPath),
					HttpMethod:     aws.String("POST"),
					HttpStatusCode: aws.Int32(200),
					ResponseBody: map[string]types.ContentBody{
						"text/plain": {
							Body: aws.String(result),
						},
					},
				},
			},
		},
	}

	fn := func(ctx context.Context, c <-chan types.InlineAgentResponseStream) {
		for event := range c {
			switch e := event.(type) {
			case *types.InlineAgentResponseStreamMemberChunk:
				resp := string(e.Value.Bytes)
				logger.Debug("[%s] %s", sessionId, resp)
				s.sessionIdChanMap[sessionId] <- resp
				s.sessionIdChanMap[sessionId] <- "TOGGLE_LOADING"

				s.invoke(ctx, sessionId, `
						Use the summary report you got earlier, respond with 10 most interesting facts using the following format for an entry:
						{
							"icon": <an emoji>,
							"title": <the statistics name>,
							"description": <the statistics description or highlight>
						}

						Focus on total (yearly) achievements instead of achievements related to a single match.

						Response only with JSON and NOTHING ELSE, start your response with [

						Use this summary of the player's League of Legends matches for the year 2024:
				`, s.responseStreamHandler(sessionId, true), nil)
			}
		}
	}
	s.invoke(ctx, sessionId, "Tool result", fn, iss)
}

func (s *service) InvokeTool(
	ctx context.Context,
	sessionId string,
	gameName string,
	tagLine string,
) ([]byte, error) {
	from := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, time.December, 31, 23, 59, 59, 999999999, time.UTC)
	scraper := scraper.New(s.riotClient)
	// Can still query any player info even from NA
	account, err := scraper.Scrape(ctx, gameName, tagLine, riot.RegionNA, from, to)
	if err != nil {
		logger.Error("failed to scrape matches %s", err)
		return nil, err
	}

	s.sessionIdChanMap[sessionId] <- "TOGGLE_LOADING"
	s.sessionIdChanMap[sessionId] <- "I have finished getting your matches, now I just need to analyze them. Please wait."
	s.sessionIdChanMap[sessionId] <- "TOGGLE_LOADING"

	// Directory containing the JSON files
	dirPath := strings.ToLower(account.GameName + "_" + account.TagLine)

	// Slice to hold all the data from JSON files
	var allData []riot.GetMatchResponse

	// Read all JSON files from the directory
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	// Loop over each file
	for _, file := range files {
		// Only process files with a .json extension
		if filepath.Ext(file.Name()) == ".json" {
			// Open the JSON file
			filePath := filepath.Join(dirPath, file.Name())
			fileData, err := ioutil.ReadFile(filePath)
			if err != nil {
				logger.Error("Error reading file %s: %v", file.Name(), err)
				continue
			}

			// Unmarshal the file data into the struct
			var data riot.GetMatchResponse
			err = json.Unmarshal(fileData, &data)
			if err != nil {
				logger.Error("Error unmarshaling file %s: %v", file.Name(), err)
				continue
			}

			// Append the data to the slice
			allData = append(allData, data)
		}
	}

	// Create directory for the player's matches
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	summary := cruncher.Crunch(account, allData)
	cruncher.GenerateMarkdown(account, allData)
	cruncher.WriteCleanedup(account, allData)

	s.batch(ctx, dirPath+"_cleanedup", awsGluePrefix)
	logger.Info("batch uploading cleaned up files for %s completed", dirPath)

	err = repairGlueTable(ctx, s.athenaClient, "taberu3", "glue-db1", fmt.Sprintf("s3://%s/repair", s3Bucket))
	if err != nil {
		logger.Error("failed to repair glue table %s", err.Error())
	}

	s.batchKb(ctx, dirPath+"_markdown")
	logger.Info("batch uploading markdown files for %s completed", dirPath)

	return summary, nil
}

func replaceSpaces(in string) string {
	return strings.ReplaceAll(in, "\n", "<br />")
}

func kaisaFix(in string) string {
	return strings.ReplaceAll(in, "\\'", "")
}

// S3
func (s *service) batch(ctx context.Context, dirPath string, targetPrefix string) error {
	// Read all JSON files from the directory
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return err
	}

	const numWorkers = 10
	jobs := make(chan S3UploadJob, len(files))
	var wg sync.WaitGroup

	// Start workers
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, i, s.s3Client, s3Bucket, jobs, &wg)
	}

	extract := func(filename string) (player string, id string, err error) {
		// Remove .json extension
		name := strings.TrimSuffix(filename, ".json")

		// Split by underscore
		parts := strings.Split(name, "_")

		// Check if we have enough parts
		if len(parts) < 5 {
			return "", "", fmt.Errorf("invalid filename format: expected at least 5 parts")
		}

		// Get the last two parts (player and ID)
		player = parts[len(parts)-2]
		id = parts[len(parts)-1]

		return player, id, nil
	}

	// Send jobs
	for _, filePath := range files {
		gn, tl, _ := extract(filePath.Name())
		key := fmt.Sprintf("%s/riotidtagline=%s/riotidgamename=%s/%s", targetPrefix, tl, gn, filepath.Base(filePath.Name()))
		jobs <- S3UploadJob{FilePath: filepath.Join(dirPath, filePath.Name()), Key: key}
	}
	close(jobs)

	// Wait for all workers
	wg.Wait()

	return nil
}

func uploadFile(ctx context.Context, client *s3.Client, bucket, filePath, key string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", key, err)
	}

	//logger.Debug("Uploaded: %s -> s3://%s/%s", filePath, bucket, key)
	return nil
}

func worker(ctx context.Context, id int, client *s3.Client, bucket string, jobs <-chan S3UploadJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		//logger.Debug("Worker %d started uploading %s", id, job.FilePath)
		if err := uploadFile(ctx, client, bucket, job.FilePath, job.Key); err != nil {
			logger.Error("Worker %d failed to upload %s: %v", id, job.FilePath, err)
		}
	}
}

// Bedrock KB
func (s *service) batchKb(ctx context.Context, dirPath string) error {
	// Read all JSON files from the directory
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read dir: %w", err)
	}

	var batch []brt.KnowledgeBaseDocument

	// Collect documents in batches of 10
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".metadata.json") {
			continue
		}

		filePath := dirPath + "/" + f.Name()

		// Add the document to the batch
		batch = append(batch, getKbDocument(filePath))

		// If batch is full (10 docs), ingest the batch
		if len(batch) == kbIngestionBatchSize {
			if err := ingestKbBatch(ctx, s.bedrockAgentClient, batch); err != nil {
				return err
			}
			// Clear the batch for the next set of documents
			batch = nil
		}
	}

	// If there are any remaining documents, ingest them in a final batch
	if len(batch) > 0 {
		if err := ingestKbBatch(ctx, s.bedrockAgentClient, batch); err != nil {
			return err
		}
	}

	return nil
}

func ingestKbBatch(ctx context.Context, client *bedrockagent.Client, batch []brt.KnowledgeBaseDocument) error {
	input := &bedrockagent.IngestKnowledgeBaseDocumentsInput{
		KnowledgeBaseId: aws.String(kbMarkdownId),
		DataSourceId:    aws.String(kbMarkdownDataSourceId),
		Documents:       batch,
		ClientToken:     aws.String(uuid.NewString()),
	}

	var lastErr error
	for attempt := 0; attempt < kbIngestionMaxRetries; attempt++ {
		_, err := client.IngestKnowledgeBaseDocuments(ctx, input)
		if err == nil {
			return nil
		}

		lastErr = err

		sleep := time.Duration(math.Pow(2, float64(attempt))) * time.Second
		logger.Warn("Throttled ingesting batch — backing off for %v", sleep)
		time.Sleep(sleep)
	}

	return fmt.Errorf("failed to ingest batch after %d retries: %w", kbIngestionMaxRetries, lastErr)
}

func getKbDocument(filename string) brt.KnowledgeBaseDocument {
	var mdJson cruncher.MetadataJSON
	mdBytes, err := os.ReadFile(filename + ".metadata.json")
	if err != nil {

	}
	err = json.Unmarshal(mdBytes, &mdJson)
	if err != nil {

	}

	markdownBytes, err := os.ReadFile(filename)
	if err != nil {

	}
	markdownBytesStr := string(markdownBytes)

	return brt.KnowledgeBaseDocument{
		Content: &brt.DocumentContent{
			DataSourceType: brt.ContentDataSourceTypeCustom,
			Custom: &brt.CustomContent{
				SourceType: brt.CustomSourceTypeInLine,
				CustomDocumentIdentifier: &brt.CustomDocumentIdentifier{
					Id: aws.String(mdJson.MetadataAttributes.MatchID + "_" + mdJson.MetadataAttributes.RiotIdGameName),
				},
				InlineContent: &brt.InlineContent{
					Type: brt.InlineContentTypeText,
					TextContent: &brt.TextContentDoc{
						Data: aws.String(markdownBytesStr),
					},
				},
			},
		},
		Metadata: &brt.DocumentMetadata{
			Type: brt.MetadataSourceTypeInLineAttribute,
			InlineAttributes: []brt.MetadataAttribute{
				{
					Key: aws.String("match_id"),
					Value: &brt.MetadataAttributeValue{
						Type:        brt.MetadataValueTypeString,
						StringValue: aws.String(mdJson.MetadataAttributes.MatchID),
					},
				},
				{
					Key: aws.String("match_month"),
					Value: &brt.MetadataAttributeValue{
						Type:        brt.MetadataValueTypeString,
						StringValue: aws.String(mdJson.MetadataAttributes.MatchMonth),
					},
				},
				{
					Key: aws.String("match_result"),
					Value: &brt.MetadataAttributeValue{
						Type:        brt.MetadataValueTypeString,
						StringValue: aws.String(mdJson.MetadataAttributes.MatchResult),
					},
				},
				{
					Key: aws.String("riotidgamename"),
					Value: &brt.MetadataAttributeValue{
						Type:        brt.MetadataValueTypeString,
						StringValue: aws.String(mdJson.MetadataAttributes.RiotIdGameName),
					},
				},
				{
					Key: aws.String("riotidtagline"),
					Value: &brt.MetadataAttributeValue{
						Type:        brt.MetadataValueTypeString,
						StringValue: aws.String(mdJson.MetadataAttributes.RiotIdTagLine),
					},
				},
			},
		},
	}
}
