# OCI container registry in Go
This is a toy application implementing the [OCI image spec] &
[OCI distribution spec] for the purposes of my own learning of Go and
containers at a fundamental level.

## TODO
* Implement [distribution endpoints]
* [Use the image-spec schema] & [validate image-spec]
* Validate server using [distribution conformance tests]

## Working notes

Printing out request URL, Content-Type & Accept headers from `docker [pull|push]`.

### Pull
* `/v2/`
* `/v2/<image>/manifests/<tag>`

```
2022/10/15 15:18:33 URL: /v2/
2022/10/15 15:18:33 URL: /v2/test/image/manifests/latest
2022/10/15 15:18:33 Accept: application/vnd.oci.image.manifest.v1+json`
2022/10/15 15:18:33 URL: /v2/test/image/manifests/latest
2022/10/15 15:18:33 Accept: application/vnd.oci.image.index.v1+json
2022/10/15 15:18:40 URL: /v2/
2022/10/15 15:18:40 URL: /v2/test/image/manifests/latest
2022/10/15 15:18:40 Accept: application/json
2022/10/15 15:18:40 URL: /v2/test/image/manifests/latest
2022/10/15 15:18:40 Accept: application/vnd.docker.distribution.manifest.v2+json
```

### Push
* `/v2/`
* `/v2/<image>/blobs/<digest>`
* `/v2/<image>/blobs/uploads/`

```
2022/10/15 11:36:49 URL: /v2/
2022/10/15 11:36:49 URL: /v2/distroless/base-debian11/blobs/sha256:bf75762436b060837307bb6b9a016fe728b10f33040c95516c621475280efc32
2022/10/15 11:36:49 URL: /v2/distroless/base-debian11/blobs/sha256:79e0d8860fadaab56c716928c84875d99ff5e13787ca3fcced10b70af29bf320
2022/10/15 11:36:49 URL: /v2/distroless/base-debian11/blobs/uploads/
```

[OCI image spec]: https://github.com/opencontainers/image-spec/blob/main/spec.md
[OCI distribution spec]: https://github.com/opencontainers/distribution-spec/blob/main/spec.md
[Use the image-spec schema]: https://github.com/opencontainers/image-spec/tree/main/specs-go/v1
[Validate image-spec]: https://github.com/opencontainers/image-spec/tree/main/schema
[distribution endpoints]: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
[distribution conformance tests]: https://github.com/opencontainers/distribution-spec/blob/main/conformance/README.md
