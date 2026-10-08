"""pytest 共享配置：把 src/ 注入 sys.path（src 布局，免安装即可跑测试）。"""

import sys
from pathlib import Path

SRC = Path(__file__).resolve().parents[1] / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))
