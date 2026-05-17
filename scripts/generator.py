#!/usr/bin/env python3

import json

i = 0
while True:
    r = {
        "method": "GET",
        "query": "/test/",
    }

    if i % 10 == 0:
        r = {
            "method": "POST",
            "query": "/post-test/",
            "body": "some data",
        }

    print(json.dumps(r))
    i += 1
