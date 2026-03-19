#!/usr/bin/env python3

import json

i = 0
while True:
    r = {
        "method": "GET",
        "query": "/api/",
        "body": "some data",
    }
    print(json.dumps(r))
    i += 1
