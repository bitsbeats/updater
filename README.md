# updater

Updater is a simple wrapper around the Kubernetes API to trigger a re-rollout of a deployment via api.

## deployment

In `manifests/` is an example deployment. Fix the namespace and modify the secret to your needs.

## configuration

Configration is done via environment variables.

- `TOKEN`: Token that must be included in the web request
- `DEPLOYMENT`: Name of the deployment that will be rolled out

## webhook

The request must contain a header with the token. Example:

```sh
curl -H "Token: $token" https://update-my-deployment.apps.example.com
```
