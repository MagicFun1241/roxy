# Roxy
##### Fast reverse proxy

## Configuration
### Head
* [dns](#configuration_dns)
* [security](#configuration_security)
  * [allowedHostsGroups](#configuration_security_allowedHostsGroups)
* [plugins](#configuration_plugins)
* [http](#configuration_http)
  * [servers](#configuration_http_servers)
  * [default](#configuration_http_default)

### <a name="configuration_dns"></a>dns
##### is string
default value: 1.1.1.1

### <a name="configuration_security"></a>security
- #### <a name="configuration_security_allowedHostsGroups"></a>allowedHostsGroups
  ##### is object with array in key values
```yaml
security:
  allowedHostsGroups:
    cloudflare:
      - 103.21.244.0/22
      - 103.22.200.0/22
      - 103.31.4.0/22
      - 104.16.0.0/12
      - 108.162.192.0/18
      - 131.0.72.0/22
      - 141.101.64.0/18
      - 162.158.0.0/15
      - 172.64.0.0/13
      - 173.245.48.0/20
      - 188.114.96.0/20
      - 190.93.240.0/20
      - 197.234.240.0/22
      - 198.41.128.0/17
    local:
      - 192.168.1.0/24
```

### <a name="configuration_plugins"></a>plugins
##### is array of plugins names
plugins directory: **"plugins"** in executable directory
```yaml
plugins:
  - ratelimit
```

### <a name="configuration_http"></a>http
- #### <a name="configuration_http_servers"></a>servers
  ##### is array of *server* type
  | Property | Type     | Description                           |
  |----------|----------|---------------------------------------|
  | name     | string   | Server name. Will not be used nowhere |
  | port     | uint16   | Port to use. Make empty if use vhost  |
  | domains  | []string | Array of domains                      |
  | routes   | []route  | Array of routes                       |
```yaml
http:
  servers:
    - name: Test
      port: 8080
      domains:
        - example.com
      routes:
        - value: /static
          to: /
```
- #### <a name="configuration_http_servers"></a>default
  | Property | Type   | Description |
  |----------|--------|-------------|
  | port     | uint16 | VHost port  |