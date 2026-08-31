#!/usr/bin/env python3
import os
import re
import socket
import sys

HOME = os.path.expanduser("~")
USER = os.environ.get("USER", "")
HOST = socket.gethostname()

patterns = [
    (re.compile(r"(?i)(https?://)[^/@\s]+@"), r"\1<credentials>@"),
    (re.compile(re.escape(HOME)), "~"),
    (re.compile(r"(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b"), "<email>"),
    (re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"), "<ip>"),
    (re.compile(r"(?i)\b(?:[0-9a-f]{2}:){5}[0-9a-f]{2}\b"), "<mac>"),
    (re.compile(r"(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+"), "Bearer <redacted>"),
    (re.compile(r"(?i)\b(authorization|password|passwd|secret|token|access_token|refresh_token|session|cookie)\s*[:=]\s*[^\s,;]+"),
     r"\1=<redacted>"),
    (re.compile(r"(?i)([?&](?:token|access_token|refresh_token|session|auth|key|secret)=)[^&#\s]+"),
     r"\1<redacted>"),
    (re.compile(r"\beyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b"), "<jwt>"),
]

if USER:
    patterns.append((re.compile(rf"(?<![A-Za-z0-9_.-]){re.escape(USER)}(?![A-Za-z0-9_.-])"), "<user>"))
if HOST:
    patterns.append((re.compile(rf"(?<![A-Za-z0-9_.-]){re.escape(HOST)}(?![A-Za-z0-9_.-])"), "<host>"))

for line in sys.stdin:
    for pattern, repl in patterns:
        line = pattern.sub(repl, line)
    sys.stdout.write(line)
