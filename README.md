# go-webdav

A WebDAV server implementation in Go with support for multiple storage backends and flexible authorization rules.

> ⚠️ Disclaimer
>
> This project is currently under active development and should be considered a work in progress. The API is in a preliminary stage and may not be stable. Please be aware that changes, including modifications, updates, or deprecations, can occur at any time without prior notice.

## Features

- Multiple filesystem backends (Local, S3, SQLite)
- Dead properties support
- WebDAV locking support
- Configurable via JSON file and environment variables
- Can be used as a library

## Usage

### As a library

> TODO

### As a server

#### Installation

```bash
go install github.com/bornholm/go-webdav/cmd/server@latest
```

Or build from source:

```bash
git clone https://github.com/bornholm/go-webdav.git
cd go-webdav
go build -o bin/server ./cmd/server
```

#### Usage

```bash
server [flags]
```

##### Command Line Flags

| Flag         | Default       | Description                          |
| ------------ | ------------- | ------------------------------------ |
| `-address`   | `:3000`       | Server listening address             |
| `-config`    | `config.json` | Path to the configuration file       |
| `-log-level` | `ERROR`       | Log level (DEBUG, INFO, WARN, ERROR) |

##### Example

```bash
# Start server  with custom config file
./server -config ./config.json

# Start server on custom port with debug logging
./server -address :8080 -log-level DEBUG -config ./config.json
```

#### Configuration

The server can be configured via a JSON configuration file and/or environment variables (prefixed with `GOWEBDAV_`).

##### Example

```json
{
  "filesystem": {
    "type": "local",
    "options": {
      "dir": "/data/webdav"
    }
  },
  "auth": {
    "enabled": true,
    "users": {
      "username_1": "password_1",
      "username_2": "password_2"
    }
  }
}
```

#### Filesystem backends

##### Local

Stores files directly on the local filesystem.

```json
{
  "filesystem": {
    "type": "local",
    "options": {
      "dir": "/path/to/webdav/root"
    }
  }
}
```

| Option | Type   | Required | Description                     |
| ------ | ------ | -------- | ------------------------------- |
| `dir`  | string | Yes      | Root directory for file storage |

##### S3

Stores files in an S3-compatible object storage service.

```json
{
  "filesystem": {
    "type": "s3",
    "options": {
      "endpoint": "s3.amazonaws.com",
      "bucket": "my-webdav-bucket",
      "user": "ACCESS_KEY_ID",
      "secret": "SECRET_ACCESS_KEY",
      "secure": true,
      "region": "us-east-1",
      "bucketLookup": "dns"
    }
  }
}
```

| Option         | Type    | Required | Default | Description                               |
| -------------- | ------- | -------- | ------- | ----------------------------------------- |
| `endpoint`     | string  | Yes      | -       | S3 endpoint URL                           |
| `bucket`       | string  | Yes      | -       | Bucket name                               |
| `user`         | string  | Yes      | -       | Access key ID                             |
| `secret`       | string  | Yes      | -       | Secret access key                         |
| `token`        | string  | No       | `""`    | Session token (for temporary credentials) |
| `secure`       | boolean | No       | `false` | Use HTTPS                                 |
| `region`       | string  | No       | `""`    | AWS region                                |
| `bucketLookup` | string  | No       | -       | Bucket lookup style: `dns` or `path`      |
| `trace`        | boolean | No       | `false` | Enable request tracing to stdout          |

##### SQLite

Stores files in a SQLite database. Useful for embedded deployments or when a single-file storage is preferred.

```json
{
  "filesystem": {
    "type": "sqlite",
    "options": {
      "path": "/data/webdav.db"
    }
  }
}
```

| Option | Type   | Required | Description                      |
| ------ | ------ | -------- | -------------------------------- |
| `path` | string | Yes      | Path to the SQLite database file |

#### Environment Variables

All configuration options can be set via environment variables with the `GOWEBDAV_` prefix. Nested options use underscores as separators.

Examples:

- `GOWEBDAV_FILESYSTEM_TYPE=local`
- `GOWEBDAV_FILESYSTEM_OPTIONS='{"dir":"/dir/path"}'`

## Development

### Running Tests

```bash
go test ./...
```

### Running Benchmarks

```bash
make benchmark
```

## License

See [LICENSE](./LICENCEq) file for details.
