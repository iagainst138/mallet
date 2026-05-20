#!/usr/bin/env python3

import json

r = {
    "method": "GET",
    "query": "/test/",
}

print(json.dumps(r))
