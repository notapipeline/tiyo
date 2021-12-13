// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package docker

// Template string for creating docker containers
const TiyoTemplate string = `FROM %s
USER root

RUN if getent passwd | grep -q 1000; then \
  if ! which userdel; then \
    deluser $(getent passwd | grep 1000 | awk -F: '{print $1}'); \
  else \
    userdel $(getent passwd | grep 1000 | awk -F: '{print $1}'); \
  fi ; \
fi

RUN if ! which useradd; then \
    adduser -S -s /bin/sh --uid 1000 -h /tiyo tiyo; \
  else \
    useradd -ms /bin/sh -u 1000 -d /tiyo tiyo; \
  fi

WORKDIR /tiyo
COPY tiyo /usr/bin/tiyo
RUN chmod 755 /usr/bin/tiyo
COPY config.json tiyo.json
RUN chmod 644 tiyo.json
USER tiyo
CMD ["/usr/bin/tiyo", "syphon"]`
