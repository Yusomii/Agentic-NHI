# 1. Establish a connection to the Virginia region
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

# 2. Get your current account ID and region (Oregon)
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# 3. The Satellite Rule in Virginia
resource "aws_cloudwatch_event_rule" "iam_global_events" {
  provider    = aws.us_east_1
  name        = "agentic-nhi-iam-global-router"
  description = "Catches global IAM events and routes them to the local region"

  event_pattern = jsonencode({
    source      = ["aws.iam"]
    detail-type = ["AWS API Call via CloudTrail"]
  })
}

# 4. The Transmitter: Beams the event to your local default bus
resource "aws_cloudwatch_event_target" "send_to_local_bus" {
  provider  = aws.us_east_1
  rule      = aws_cloudwatch_event_rule.iam_global_events.name
  target_id = "SendToLocalBus"
  arn       = "arn:aws:events:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:event-bus/default"
  role_arn  = aws_iam_role.eventbus_cross_region_role.arn
}

# 5. The IAM Permission to transmit across regions
resource "aws_iam_role" "eventbus_cross_region_role" {
  name = "agentic-nhi-eb-cross-region"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "events.amazonaws.com" }
    }]
  })
}

resource "aws_iam_policy" "eventbus_cross_region_policy" {
  name = "agentic-nhi-eb-cross-region-policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action   = "events:PutEvents"
      Effect   = "Allow"
      Resource = "arn:aws:events:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:event-bus/default"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eb_cr_attach" {
  role       = aws_iam_role.eventbus_cross_region_role.name
  policy_arn = aws_iam_policy.eventbus_cross_region_policy.arn
}