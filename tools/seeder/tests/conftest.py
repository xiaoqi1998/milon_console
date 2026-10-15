# -*- coding: utf-8 -*-
"""让 tests 能直接 import seedlib 与 seed 入口。"""
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)
