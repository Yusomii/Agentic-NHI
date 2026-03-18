# Agentic-NHI Commander

An event-driven, Zero-Trust AWS security engine designed to detect, analyze, and mitigate anomalous Non-Human Identity (NHI) behavior in real-time using Foundation Models.

## 🏗 Architecture Overview

Agentic-NHI is built on a serverless, Hexagonal Architecture pattern. It intercepts raw AWS management events, processes them through an AI reasoning engine (Claude via Amazon Bedrock), and initiates a Human-in-the-Loop (HITL) authorization flow via Slack.

### The Data Flow
1. **Ingress:** AWS CloudTrail captures IAM activity (e.g., `CreateUser`, `AttachRolePolicy`).
2. **Event Routing:** Amazon EventBridge filters high-risk events and triggers the Commander Lambda.
3. **Reasoning Engine:** The Go binary reconstructs the event context and queries Amazon Bedrock (Claude 3.5 Sonnet) to determine the malicious intent or blast radius of the action.
4. **Human-in-the-Loop:** If the AI determines the action is anomalous, it pushes an interactive Block Kit payload to a secure Slack channel for immediate SecOps approval/denial.

## 🛠 Tech Stack
* **Core Application:** Go (1.22)
* **Infrastructure as Code (IaC):** HashiCorp Terraform
* **Cloud Provider:** Amazon Web Services (AWS)
* **Compute & Security:** AWS Lambda, IAM (Least-Privilege Execution)
* **AI/ML:** Amazon Bedrock (Claude 3.5 Sonnet)
* **CI/CD:** GitHub Actions (Zero-Trust OIDC Federation)

## 🔒 Security Posture & DevSecOps

This project adheres to strict enterprise security standards:
* **Zero-Trust Deployment:** The GitHub Actions pipeline does **not** use long-lived AWS Access Keys. It relies entirely on an OpenID Connect (OIDC) bridge, requesting temporary, short-lived STS tokens for Terraform deployments.
* **Ephemeral Compute:** The Go binary runs in a transient AWS Lambda environment, meaning there are no permanent servers to patch or expose to edge vulnerabilities.
* **Least-Privilege IAM:** The Lambda execution role only contains the exact `bedrock:InvokeModel` and `logs:PutLogEvents` permissions required to operate.
* **Secret Management:** Slack OAuth tokens are injected securely at runtime via Terraform variables mapped to GitHub Actions encrypted secrets.

## 🚀 Deployment Pipeline

The infrastructure is fully automated. Pushing to the `main` branch triggers the GitHub Actions workflow, which:
1. Provisions an Ubuntu runner.
2. Cross-compiles the Go application for `linux/arm64`.
3. Assumes the AWS OIDC deployment role.
4. Executes `terraform init` and `terraform apply` to synchronize the cloud state.