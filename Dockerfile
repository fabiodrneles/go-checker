# Built by GoReleaser (dockers_v2): the binaries come from the build, per platform.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/go-checker /usr/bin/go-checker
EXPOSE 9090
ENTRYPOINT ["/usr/bin/go-checker"]
CMD ["--help"]
