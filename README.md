# Overview

I was looking for a tool for load test a rserve server and didn't find anything so I did the minimal work to fit my need.

# Config

<pre>
http_post:
  host: "http://localhost"
  path: "/"
  data: ""
rserve:
  host: "localhost"
  port: 6346
  data: "print('hi')"
</pre>

# Run

<pre>
./load-test -freq 10 -config config.yaml
</pre>

# TODO

1. threads
2. put them in different packages
3. make the code more extensible 