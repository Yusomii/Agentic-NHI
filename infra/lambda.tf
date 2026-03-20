variable "slack_bot_token" {
  type      = string
  sensitive = true
}

variable "slack_channel_id" {
  type      = string
  sensitive = true
}

# 1. Deployment Packages (Pointing to subfolders)
data "archive_file" "commander_zip" {
  type        = "zip"
  source_file = "${path.module}/../bin/commander/bootstrap"
  output_path = "${path.module}/commander_payload.zip"
}

data "archive_file" "executioner_zip" {
  type        = "zip"
  source_file = "${path.module}/../bin/executioner/bootstrap"
  output_path = "${path.module}/executioner_payload.zip"
}

# 2. Unified IAM Role
resource "aws_iam_role" "iam_for_lambda" {
  name = "agentic-nhi-execution-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "lambda_policy" {
  name = "agentic-nhi-lambda-policy"
  role = aws_iam_role.iam_for_lambda.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "iam:UpdateAccessKey"
      ]
      Effect   = "Allow"
      Resource = "*"
    }]
  })
}

# 3. Commander Function
resource "aws_lambda_function" "commander" {
  filename         = data.archive_file.commander_zip.output_path
  source_code_hash = data.archive_file.commander_zip.output_base64sha256
  function_name    = "agentic-nhi-commander"
  role             = aws_iam_role.iam_for_lambda.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 256

  environment {
    variables = {
      SLACK_BOT_TOKEN  = var.slack_bot_token
      SLACK_CHANNEL_ID = var.slack_channel_id
    }
  }
}

# 4. Executioner Function
resource "aws_lambda_function" "executioner" {
  filename         = data.archive_file.executioner_zip.output_path
  source_code_hash = data.archive_file.executioner_zip.output_base64sha256
  function_name    = "agentic-nhi-executioner"
  role             = aws_iam_role.iam_for_lambda.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 256
}

# 5. API Gateway (The "Front Door")
resource "aws_apigatewayv2_api" "slack_webhook" {
  name          = "agentic-nhi-api"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_stage" "default_stage" {
  api_id      = aws_apigatewayv2_api.slack_webhook.id
  name        = "$default"
  auto_deploy = true
}

resource "aws_apigatewayv2_integration" "lambda_integration" {
  api_id             = aws_apigatewayv2_api.slack_webhook.id
  integration_uri    = aws_lambda_function.executioner.invoke_arn
  integration_type   = "AWS_PROXY"
  integration_method = "POST"
}

resource "aws_apigatewayv2_route" "webhook_route" {
  api_id    = aws_apigatewayv2_api.slack_webhook.id
  route_key = "POST /webhook"
  target    = "integrations/${aws_apigatewayv2_integration.lambda_integration.id}"
}

# 6. Gateway Permission (Dynamically linked to the API ID)
resource "aws_lambda_permission" "apigw_invoke" {
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.executioner.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.slack_webhook.execution_arn}/*/*"
}

output "slack_webhook_url" {
  value = "${aws_apigatewayv2_api.slack_webhook.api_endpoint}/webhook"
}

# 1. The Local Rule (The "Catcher")
resource "aws_cloudwatch_event_rule" "local_processor" {
  name        = "agentic-nhi-local-processor"
  description = "Catches IAM events forwarded from us-east-1"
  event_pattern = jsonencode({
    source      = ["aws.iam"]
    detail-type = ["AWS API Call via CloudTrail"]
  })
}

# 2. The Target (Link the Catcher to the Commander)
resource "aws_cloudwatch_event_target" "commander_target" {
  rule      = aws_cloudwatch_event_rule.local_processor.name
  target_id = "TriggerCommander"
  arn       = aws_lambda_function.commander.arn
}

# 3. The Permission (Allow EventBridge to wake up the Commander)
resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.commander.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.local_processor.arn
}
# --- GitHub Actions CI/CD Deployment Trust ---

# 1. Establish GitHub as a trusted Identity Provider
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  # Official GitHub OIDC Thumbprints
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1", "1c58a3a8518e8759bf075b76b750d4f2df264fcd"] 
}

# 2. Create the Role GitHub will assume
resource "aws_iam_role" "github_actions" {
  name = "agentic-nhi-github-actions"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = { "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com" }
        # RESTRICTS ACCESS TO ONLY YOUR REPOSITORY
        StringLike   = { "token.actions.githubusercontent.com:sub" = "repo:Yusomii/Agentic-NHI:*" }
      }
    }]
  })
}

# 3. Grant Terraform the permissions it needs to deploy
resource "aws_iam_role_policy_attachment" "gh_actions_admin" {
  role       = aws_iam_role.github_actions.name
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess"
}

output "github_role_arn" {
  value = aws_iam_role.github_actions.arn
}