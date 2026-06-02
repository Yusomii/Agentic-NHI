# Agentic-NHI: Event-Driven Zero-Trust Security Orchestrator

An enterprise-grade, event-driven AWS security orchestrator written in Go. Agentic-NHI automatically ingests anomalous CloudTrail IAM telemetry, evaluates threat vectors utilizing Amazon Bedrock (Claude 4.6 Sonnet), and enforces a strict Human-in-the-Loop (HITL) approval workflow via cryptographically verified Slack webhooks before executing programmatic remediation.

## 🏗 Decoupled Architecture & Data Flow

```mermaid
graph TD
    %% Ingestion & Decoupling Layer
    A[AWS CloudTrail] -->|Anomalous IAM Events| B(Amazon EventBridge)
    B -->|Route Pattern| C[(Amazon SQS Buffer)]
    C -.->|Failure Fallback| DLQ[(Dead Letter Queue)]

    %% Analysis Plane (Least Privilege: Read Only)
    C -->|Trigger Batch| D{Commander Lambda}
    D <-->|Evaluate Threat| E[Amazon Bedrock: Claude 4.6 Sonnet]
    D -->|Dispatch Context| F((Slack Webhook))

    %% Human-in-the-Loop
    F -->|Review Diff| G[Human SecOps]

    %% Execution Plane (Isolated Blast Radius)
    G -->|Cryptographic Approval| H[Amazon API Gateway]
    H -->|Invoke| I{Executioner Lambda}
    I -->|Remediate| J[AWS IAM Controls]
```

### The Telemetry Pipeline

1. **Ingress & Buffering:** AWS CloudTrail captures high-risk IAM activity. EventBridge routes these payloads into an Amazon SQS buffer, decoupling ingestion velocity from compute processing limits.
2. **The Analysis Plane (Commander):** A Go-based AWS Lambda consumes the SQS batches. It reconstructs the event context and queries Amazon Bedrock to determine the blast radius of the anomalous action. This execution role has zero infrastructure write permissions.
3. **Human-in-the-Loop (HITL):** If the reasoning engine classifies the action as a critical threat, it dispatches an interactive Block Kit payload to a secure Slack channel for SecOps review.
4. **The Execution Plane (Executioner):** SecOps approval triggers a callback to an Amazon API Gateway. The Gateway validates the cryptographic Slack HMAC signature and invokes the isolated Executioner Lambda to isolate the compromised IAM entity.

## 🔒 Security Posture & DevSecOps Guardrails

This architecture is designed to map against rigorous compliance frameworks (e.g., NIST 800-171, CMMC) by enforcing zero-trust principles at every layer:

* **Zero-Trust CI/CD (OIDC):** The GitHub Actions pipeline explicitly denies the use of long-lived AWS Access Keys. It relies on an OpenID Connect (OIDC) federation trust, requesting temporary, short-lived STS tokens to execute `terraform apply`.
* **Blast Radius Isolation (Dual-Compute):** The application is split into two distinct binaries (`cmd/commander` and `cmd/executioner`). If the logic plane is compromised, the attacker still lacks the IAM permissions to alter infrastructure.
* **Cryptographic Perimeter Defense:** The remediation execution endpoint is shielded by an API Gateway that enforces strict token-bucket rate limits and requires SHA256 header signature validation, dropping unauthorized payloads before compute initialization.
* **Immutable Infrastructure:** 100% of the cloud topology (Queues, EventBridge rules, API Gateways, IAM Roles) is defined deterministically via HashiCorp Terraform (`infra/`).

## ⚙️ System Reliability & Evolved Architecture

### Resolving State-Lock Contention via SQS Decoupling

* **The Vulnerability (v1):** The initial synchronous architecture triggered EventBridge to invoke Lambda directly. During a simulated credential-stuffing attack, concurrent telemetry floods caused extreme DynamoDB write-throttling and permanent alert dropping due to API rate limits.
* **The Patch (v2):** The ingestion pipeline was re-architected to implement Distributed Eventual Consistency. By injecting an Amazon SQS load-leveling buffer and a Dead Letter Queue (DLQ), ingestion is mathematically decoupled from compute. Traffic bursts are safely queued, eliminating state-lock contention and guaranteeing telemetry persistence.

### Resolving Wildcard Permissions via Role Bifurcation

* **The Vulnerability (v1):** Early iterations utilized a single monolithic Lambda execution role containing broad IAM wildcards to handle both Bedrock inference and IAM remediation.
* **The Patch (v2):** Strict Least-Privilege was enforced. The Commander role only possesses `bedrock:InvokeModel` and `sqs:ReceiveMessage`. The Executioner role operates in a completely detached security group with tightly scoped IAM revocation permissions.

## 🛠 Tech Stack Core

* **Systems Programming:** Go 1.22 (Statically linked `linux/arm64` binaries)
* **Infrastructure as Code:** HashiCorp Terraform (HCL)
* **Cloud Primitives:** AWS (Lambda, SQS, API Gateway, EventBridge, IAM)
* **AI/Inference:** Amazon Bedrock (Anthropic Claude 3.5 Sonnet)
* **Deployment Automation:** GitHub Actions

## 🚀 Automated Deployment

Deployments are strictly pipeline-driven to prevent configuration drift. Pushing to `main` triggers the CI/CD workflow:

1. Provisions an isolated Ubuntu build runner.
2. Cross-compiles the dual Go binaries.
3. Authenticates via short-lived OIDC federated tokens.
4. Executes `terraform init` and `terraform apply` to synchronize the AWS environment.

```

```