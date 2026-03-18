variable "slack_bot_token" {
  type      = string
  sensitive = true
}

variable "slack_channel_id" {
  type      = string
  sensitive = true
}

data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../bin/bootstrap"
  output_path = "${path.module}/lambda_payload.zip"
}

resource "aws_iam_role" "iam_for_lambda" {
  name = "agentic-nhi-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy" "lambda_policy" {
  name = "agentic-nhi-lambda-policy"
  role = aws_iam_role.iam_for_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "bedrock:InvokeModel"
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
  })
}

resource "aws_lambda_function" "commander" {
  filename      = data.archive_file.lambda_zip.output_path
  function_name = "agentic-nhi-commander"
  role          = aws_iam_role.iam_for_lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  environment {
    variables = {
      SLACK_BOT_TOKEN  = var.slack_bot_token
      SLACK_CHANNEL_ID = var.slack_channel_id
    }
  }
}