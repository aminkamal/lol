package service

const (
	OAPI = `
{
  "openapi": "3.0.0",
  "info": {
    "title": "Riot API Tool",
    "version": "1.0.0"
  },
  "paths": {
    "/scrape": {
      "post": {
        "summary": "Get matches for for a user",
        "description": "Get matches for a user",
        "operationId": "getData",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "gamename": {
                    "type": "string",
                    "description": "Name"
                  },
                  "tagLine": {
                    "type": "string",
                    "description": "Tag line"
                  }
                },
                "required": ["gamename", "tagLine"]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}
`
)
