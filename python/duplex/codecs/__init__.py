
import importlib

def load(name):
    return importlib.import_module(
        "."+name, "duplex.codecs").codec
