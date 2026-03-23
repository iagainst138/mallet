#!/usr/bin/env python3

import json

i = 0
while True:
    r = {
        "method": "GET",
        "query": "/test/",
        "body": "some data",
    }

    if i%10 == 0:
        r["method"] = "POST"

    print(json.dumps(r))
    i += 1
