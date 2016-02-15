import sys
import pprint
from asyncio import *
def asynchook(v):
  import builtins
  if iscoroutine(v):
    builtins._ = get_event_loop().run_until_complete(v)
    pprint.pprint(builtins._)
  else:
    sys.__displayhook__(v)
sys.displayhook = asynchook
