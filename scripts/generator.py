#!/usr/bin/env python3

import json
import time


#with open("/tmp/sd.log", "w") as f:
#    for i in range(0, 20):
#        print(i, file=sys.stderr)
#        f.write(f"{i}\n")
#        s = f"{i}"*10
#        sys.stdout.write(f"{s}\n")
#        sys.stdout.flush()
#        f.flush()
#        time.sleep(0.1)

#for i in range(0, 20):
i = 0
while True:
    r = {
        "method": "GET",
        "query": f"/api/",#?id={i}",
        "body": "some data",
    }
    print(json.dumps(r))
    i += 1
    #time.sleep(0.5)
