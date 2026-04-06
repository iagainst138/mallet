#!/usr/bin/env python3

import json

r = {
    "method": "GET",
    "query": "/test/",
    "body": "some data",
}

print(json.dumps(r))
