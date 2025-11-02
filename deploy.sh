#!/bin/bash

INSTANCE_ID="i-123456789"
LOCAL_BINARY="./ritoapi"
REMOTE_PATH="/home/ubuntu/ritoapi"
SERVICE_NAME="ritoapi"
S3_BUCKET="<DEPLOY_BUCKET>"

echo "🚀 Starting deployment via SSM..."

# Compile app and uploading static files
echo "🔥 Compiling arm64..."
GOOS=linux GOARCH=arm64 go build -o ritoapi main.go
scp -r templates/ ubuntu@123.123.123.123:/home/ubuntu

# Upload binary via S3
echo "📦 Uploading binary to S3..."

aws s3 cp "$LOCAL_BINARY" "s3://${S3_BUCKET}/ritoapi"

if [ $? -ne 0 ]; then
    echo "❌ S3 upload failed"
    exit 1
fi

echo "✅ Upload successful"

# Download on instance, create systemd service, and restart
echo "🔄 Deploying and configuring service..."
COMMAND_ID=$(aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
        "echo \"Downloading binary...\"",
        "aws s3 cp s3://'"${S3_BUCKET}"'/ritoapi /tmp/ritoapi.new",
        "echo \"Creating systemd service file...\"",
        "sudo tee /etc/systemd/system/'"${SERVICE_NAME}"'.service > /dev/null <<EOF",
        "[Unit]",
        "Description=RiftRewind",
        "After=network.target",
        "",
        "[Service]",
        "Type=simple",
        "User=ubuntu",
        "WorkingDirectory=/home/ubuntu",
        "ExecStart='"${REMOTE_PATH}"'",
        "Restart=always",
        "RestartSec=5",
        "StandardOutput=journal",
        "StandardError=journal",
        "",
        "[Install]",
        "WantedBy=multi-user.target",
        "EOF",
        "echo \"Stopping service if running...\"",
        "sudo systemctl stop '"${SERVICE_NAME}"' 2>/dev/null || true",
        "echo \"Moving binary into place...\"",
        "mv /tmp/ritoapi.new '"${REMOTE_PATH}"'",
        "chmod +x '"${REMOTE_PATH}"'",
        "echo \"Reloading systemd and enabling service...\"",
        "sudo systemctl daemon-reload",
        "sudo systemctl enable '"${SERVICE_NAME}"'",
        "sudo systemctl start '"${SERVICE_NAME}"'",
        "echo \"Checking service status...\"",
        "sudo systemctl status '"${SERVICE_NAME}"' --no-pager"
    ]' \
    --output text \
    --query 'Command.CommandId')

echo "📋 Command ID: $COMMAND_ID"
echo "⏳ Waiting for command to complete..."

# Wait for command to finish
aws ssm wait command-executed \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID"

# Get command output
echo ""
echo "📄 Command Output:"
aws ssm get-command-invocation \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --query 'StandardOutputContent' \
    --output text

echo ""
echo "✅ Deployment complete!"
