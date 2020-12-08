# import-deploy

Manage instances of imports stored in [import-repository](https://github.com/SENERGY-Platform/import-repository)

## Backends

import-deploy can manage containers in three different backends: 
*   Docker daemon
*   Rancher (v1)
*   Rancher (v2)

## Config

Simply set these environment variables (default values in brackets):
*    SERVER_PORT: port to listen on (8080)
*    JWT_PUB_RSA: public RSA Key to validate JWTs. If not set, JWTs will not be validated ("")
*    FORCE_AUTH: whether to enforce authentication (true)
*    FORCE_USER: whether to enforce a user id in the JWT (true)
*    MONGO_URL: URL of the mongo db (mongodb://localhost:27017)
*    MONGO_TABLE: mongo db table to use (importdeploy)
*    MONGO_IMPORT_TYPE_COLLECTION: mongo collection to use (instances)
*    MONGO_REPL_SET: whether the mongo db is running as replication set (true)
*    IMPORT_REPO_URL: URL of the [import-repository](https://github.com/SENERGY-Platform/import-repository) (http://localhost:8181)
*    PERMISSIONS_URL: URL of the [permission-search](https://github.com/SENERGY-Platform/permission-search) (http://permissionsearch:8080)
*    KAFKA_BOOTSTRAP: address of the kafka broker (localhost:9092)
*    DEPLOY_MODE: which backend to use (docker)
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
*    DEBUG: whether to print debug output (true)

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
  "owner": string
}
```

service_id and owner are hidden from the user. id, image and kafka_topic may not be set manually.

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

