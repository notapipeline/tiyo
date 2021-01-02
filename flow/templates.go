package flow

// Arguments: [org/]container, version, pipeline
const dockerTemplate string = `FROM %s:%s
USER root
RUN mkdir /tiyo
WORKDIR /tiyo
COPY tiyo /usr/bin/tiyo
COPY config.json .
RUN chmod +x /usr/bin/tiyo
CMD ["/usr/bin/tiyo", "syphon"]`
