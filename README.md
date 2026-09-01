# import-deploy

Manage instances of imports stored in [import-repository](https://github.com/SENERGY-Platform/import-repository)

## Backends

import-deploy can manage containers in three different backends: 
*   Docker daemon
*   Rancher (v1)
*   Rancher (v2)

## Config

Simply set these environment variables (default values in brackets):
* SERVER_PORT: port to listen on (8080)
* JWT_PUB_RSA: public RSA Key to validate JWTs ("")
* MONGO_URL: URL of the mongo db (mongodb://localhost:27017)
* MONGO_TABLE: mongo db table to use (importdeploy)
* MONGO_IMPORT_TYPE_COLLECTION: mongo collection to use (instances)
* MONGO_REPL_SET: whether the mongo db is running as replication set (true)
* IMPORT_REPO_URL: URL of the [import-repository](https://github.com/SENERGY-Platform/import-repository) (http://localhost:8181)
* PERMISSIONS_URL: URL of the [permission-search](https://github.com/SENERGY-Platform/permission-search) (http://permissionsearch:8080)
* KAFKA_BOOTSTRAP: address of the kafka broker (localhost:9092)
* KAFKA_REPLICATION: number of replicas for newly created topics (1)
* STARTUP_ENSURE_DEPLOYED: if true, will recreate any missing instances at startup (false)
* DEPLOY_MODE: which backend to use (docker)
* docker
  * DOCKER_NETWORK: network to start containers in (bridge)
  * DOCKER_PULL: whether to pull images before starting containers (true)
  * DOCKER_HOST: url to the docker server (/var/run/docker.sock)
  * DOCKER_API_VERSION: Docker api version (latest)
  * DOCKER_CERT_PATH: location of docker TLS certificates ("")
  * DOCKER_TLS_VERIFY: whether to check TLS certificates (false)
* rancher1
  * RANCHER_URL: API endpoint of rancher (http://rancher/v2-beta/projects/___/)
  * RANCHER_ACCESS_KEY: Rancher API key ("")
  * RANCHER_SECRET_KEY: Secret of rancher API key ("")
  * RANCHER_STACK_ID: stack to deploy containers in ("")
* rancher2
  * RANCHER_URL: API endpoint of rancher (https://rancher/v3/)
  * RANCHER_ACCESS_KEY: Rancher API key ("")
  * RANCHER_SECRET_KEY: Secret of rancher API key ("")
  * RANCHER_PROJECT_ID: project to deploy containers in ("") 
  * RANCHER_NAMESPACE_ID: namespace to deploy containers in ("")
* DEBUG: whether to print debug output (true)
* OTEL_ENDPOINT: OTLP collector traces are exported to ("", meaning the in-cluster Jaeger)

## Data model

### InstanceConfig
```
{
  "name": string,  
  "value": any 
}
```

### Instance
```
{
  "id": string,
  "name": string,
  "import_type_id": string,
  "image": string,
  "kafka_topic": string,
  "configs": InstanceConfig[],
  "restart": bool,
  "service_id": string.
  "owner": string,
  "generated": bool,  
  "created_at": string,
  "updated_at": string,
  "baggage": {string: string}
}
```

service_id and owner are hidden from the user. id, image and kafka_topic may not be set manually.

## OpenTelemetry

An incoming request brings its trace context and its baggage with it, and the baggage
of the request that created an instance is stored on that instance. It reaches the
import container twice, because the two consumers differ: as pod labels, which the log
aggregation attaches to every container log line, and as the BAGGAGE environment
variable, which [import-lib](https://github.com/SENERGY-Platform/import-lib) reads to
put the same fields into the import's own log records. A caller that sends its smart
service instance id this way finds every log line of the resulting import under it.

The instance's own id joins the baggage as `import_id` once it has been generated.
Labels are a best-effort index: a value Kubernetes would refuse -- over 63 characters,
or a username that is an email address -- is left out of them rather than sanitized,
while the environment variable carries the complete baggage. The pod labels are
prefixed with `baggage.senergy.infai.org/`; the workload's own labels and the
deployment selector stay as they were.

A value sent in the `baggage` field of a request body is ignored. Docker mode gets the
environment variable only: there are no pod labels to read there.

## API

### Create
```
POST /instances
Body: Instance without id, kafka_topic, image and owner (set automatically)
```

### Read
```
GET /instances/:id
Returns the full Instance
```

### List
```
GET /instances
Returns a list of Instances
Query parameters:
* search: filter by name
* limit: limit returned instances (default: 100)
* offset: offset for pagination (default: 0)
* sort: field.(asc|desc) for ordering instances (default: name.asc)
* exclude_generated: if set to "true" generated instances will be excluded.
```

### Update
```
PUT /instances/:id
Body: Full ImportType. Ensure id in url and ImportType match. Changing owner or kafka_topic is not allowed.
```

### Delete
```
DELETE /instances/:id
```

## Security
Identity is provided by populating the Header "Authorization" with a JWT (prefixed by "Bearer ").
The token can be validated by providing a public RSA key as config.
When creating or updating an instance, read and execute access are checked at [import-repository](https://github.com/SENERGY-Platform/import-repository)
and [permission-search](https://github.com/SENERGY-Platform/permission-search)

## Interactions with [import-repository](https://github.com/SENERGY-Platform/import-repository)
When creating or updating an instance, the referenced import_type will be read from the [import-repository](https://github.com/SENERGY-Platform/import-repository).
This ensures read access to the import_type and provides default values for image, restart and configs.

