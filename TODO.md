# Aspirational Features

- Keep a tfplan log & make tf plans available for review
- Allow branch, tag and PR workflows
- Manage terraform state
	- Save working directory (.terraform + state)
	- Allow state locking for racing conditions
	- Set workspace according to [env].tfvars (or set TF_WORKSPACE)
- Read Secrets / ConfigMap based on TF_WORKSPACE
- Wait for apply approval event (cases for critical infrastructure)
- Cache Provider Plugins

https://learn.hashicorp.com/terraform/development/running-terraform-in-automation
