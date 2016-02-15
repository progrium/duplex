from . import protocol
from . import async
from . import sync
from . import codecs

__all__ = (
    "RPC",
    "protocol",
)

def RPC(*args, **kwargs):
    if "async" in kwargs:
        amode = kwargs["async"]
        del kwargs["async"]
    else:
        amode = True
    if amode:
        return async.RPC(*args, **kwargs)
    else:
        return sync.RPC(*args, **kwargs)


from .version import version as __version__
