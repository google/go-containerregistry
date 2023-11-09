## crane auth token

Retrieves a token for a remote repo

```
crane auth token REPO [flags]
```

### Examples

```
# If you wanted to mount a blob from debian to ubuntu.
$ curl -H "$(crane auth token -H --push --mount debian ubuntu)" ...

# To get the raw list tags response
$ curl -H "$(crane auth token -H ubuntu)" https://index.docker.io/v2/library/ubuntu/tags/list

```

### Options

```
  -H, --header          Output in header format
  -h, --help            help for token
  -m, --mount strings   Scopes to mount from
      --push            Request push scopes
```

### Options inherited from parent commands

```
      --allow-nondistributable-artifacts   Allow pushing non-distributable (foreign) layers
      --insecure                           Allow image references to be fetched without TLS
      --platform platform                  Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)
  -v, --verbose                            Enable debug logs
```

### SEE ALSO

* [crane auth](crane_auth.md)	 - Log in or access credentials

