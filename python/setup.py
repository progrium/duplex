import os
import sys

import setuptools

# Avoid polluting the .tar.gz with ._* files under Mac OS X
os.putenv('COPYFILE_DISABLE', 'true')

root = os.path.dirname(__file__)

# Prevent distutils from complaining that a standard file wasn't found
README = os.path.join(root, 'README')
if not os.path.exists(README):
    os.symlink(README + '.md', README)

description = "Full duplex RPC and service framework"

with open(os.path.join(root, 'README'), encoding='utf-8') as f:
    long_description = '\n\n'.join(f.read().split('\n\n')[1:])

with open(os.path.join(root, 'duplex', 'version.py'), encoding='utf-8') as f:
    exec(f.read())

py_version = sys.version_info[:2]

if py_version < (3, 3):
    raise Exception("duplex requires Python >= 3.3.")

setuptools.setup(
    name='duplex',
    version=version,
    author='Jeff Lindsay',
    author_email='progrium@gmail.com',
    url='https://github.com/progrium/duplex',
    description=description,
    long_description=long_description,
    #download_url='https://pypi.python.org/pypi/duplex',
    packages=[
        'duplex',
    ],
    extras_require={
        ':python_version=="3.3"': ['asyncio'],
    },
    classifiers=[
        # "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.3",
        "Programming Language :: Python :: 3.4",
        "Programming Language :: Python :: 3.5",
    ],
    platforms='all',
    license='MIT'
)
