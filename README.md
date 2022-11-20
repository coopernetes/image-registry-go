# OCI container registry in Go
This is a toy application implementing the [OCI image spec] &
[OCI distribution spec] for the purposes of my own learning of Go and
containers at a fundamental level.

## TODO
* Implement [distribution endpoints]
* [Use the image-spec schema] & [validate image-spec]
* Validate server using [distribution conformance tests]

[OCI image spec]: https://github.com/opencontainers/image-spec/blob/main/spec.md
[OCI distribution spec]: https://github.com/opencontainers/distribution-spec/blob/main/spec.md
[Use the image-spec schema]: https://github.com/opencontainers/image-spec/tree/main/specs-go/v1
[Validate image-spec]: https://github.com/opencontainers/image-spec/tree/main/schema
[distribution endpoints]: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
[distribution conformance tests]: https://github.com/opencontainers/distribution-spec/blob/main/conformance/README.md
