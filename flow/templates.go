package flow

// Arguments: [org/]container, version, pipeline
const dockerTemplate string = `FROM %s:%s
RUN mkdir /tiyo
WORKDIR /tiyo
COPY tiyo /usr/bin/tiyo
COPY config.json .
CMD ["/usr/bin/tiyo", "watch", "-p", "%s"]`
