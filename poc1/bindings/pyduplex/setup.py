#!/usr/bin/env python

import os
from setuptools import setup, find_packages

setup(
        name='pyduplex',
        version='0.0.1',
        packages=find_packages(),
        author='Robert Xu',
        author_email='robxu9@gmail.com',
        description='Python API to the Duplex message system',
        long_description=open(os.path.join(os.path.dirname(__file__), 'README.md')).read(),
        license='BSD',
        keywords='message queue duplex dpx linux',
        platforms=['Linux'],
        classifiers=[
            'Development Status :: 2 - Pre-Alpha',
            'Indended Audience :: Developers',
            'License :: OSI Approved :: BSD License',
            'Operating System :: POSIX',
            'Programming Language :: Python',
            'Topic :: Software Development :: Libraries :: Python Modules',
        ]
)
