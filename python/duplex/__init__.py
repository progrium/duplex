# This relies on each of the submodules having an __all__ variable.

from .duplex import *

__all__ = (
    duplex.__all__
)

from .version import version as __version__
