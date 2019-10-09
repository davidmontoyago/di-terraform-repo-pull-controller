# Aspirational Features

- Seed a repo
- Scan repo
	- Schedule job run and plan changes
	- Keep a tfplan log
	- Fetch a tfplan and apply
	- Report current state/last applied
	- Set TF_IN_AUTOMATION
	- Allow branch, tag and PR workflows
- Manage terraform state
	- Save working directory (.terraform + state)
	- Allow state locking for racing conditions
	- Set workspace according to [env].tfvars (or set TF_WORKSPACE)
- Read Secrets / ConfigMap based on TF_WORKSPACE
- Wait for apply approval event (cases for critical infrastructure)
- Cache Provider Plugins

https://learn.hashicorp.com/terraform/development/running-terraform-in-automation
