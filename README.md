# Roxy
###### Fast reverse proxy

## Configuration
### Head
* [dns](#configuration_dns)
* [security](#configuration_security)
  * [allowedHostsGroups](#configuration_security_allowedHostsGroups)
* [plugins](#configuration_plugins)
* [http](#configuration_http)
  * [servers](#configuration_http_servers)
  * [default](#configuration_http_default)

### <a name="configuration_http"></a>http
- #### <a name="configuration_http_servers"></a>servers
  ##### is array of *server* type
  | Property | Type     | Description                           |
  |----------|----------|---------------------------------------|
  | name     | string   | Server name. Will not be used nowhere |
  | port     | uint16   | Port to use. Make empty if use vhost  |
  | domains  | []string | Array of domains                      |
  | routes   | []route  | Array of routes                       |
- #### <a name="configuration_http_servers"></a>default
  | Property | Type   | Description |
  |----------|--------|-------------|
  | port     | uint16 | VHost port  |