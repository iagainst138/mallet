#!/usr/bin/env python3

import json

requests = [
    {
        "method": "GET",
        "query": "/test/",
    },
    {
        "method": "GET",
        "query": "/args/?s=d",
    },
    {
        "method": "GET",
        "query": "/headers/?header=x-test&value=testing123",
        "headers": {
            "x-test": "testing123",
        },
    },
    {
        "method": "PUT",
        "query": "/put-test/",
        "body": "some data",
    },
    {
        "method": "POST",
        "query": "/post-test/",
        "body": "some data",
    }
]

while True:
    for r in requests:
        print(json.dumps(r))
