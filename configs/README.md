# Configuration direction

No application configuration loader exists yet. This directory will hold documented, sanitized examples for application settings, PostgreSQL connectivity, Seqera integration, workflow/version allowlists, policy limits, and environment-specific settings.

Configuration affecting execution or authorization must be versioned or referenced in the run's review evidence. Separate credentials from execution-relevant configuration; changing a compute profile must not silently change approved intent. Define resource units and task/run scope explicitly.

Never commit secrets or production credentials. Use environment injection or a managed secret store later. Keep safe example files in source control, with clearly nonfunctional placeholders and no real dataset locations. Local `.env` files and secret directories are ignored. Configuration must not allow client-provided values to bypass server-side policy.
