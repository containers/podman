#!/usr/bin/env python

import os

from setuptools import find_packages, setup


root = os.path.abspath(os.path.dirname(__file__))

with open(os.path.join(root, 'README.md')) as me:
    readme = me.read()

with open(os.path.join(root, 'requirements.txt')) as r:
    requirements = r.read().splitlines()


setup(
    name='podman',
    version=os.environ.get('PODMAN_VERSION', '0.0.0'),
    description='A library for communicating with a Podman server',
    author='Jhon Honce',
    author_email='jhonce@redhat.com',
    license='Apache Software License',
    long_description=readme,
    include_package_data=True,
    install_requires=requirements,
    packages=find_packages(exclude=['test']),
    python_requires='>=3',
    zip_safe=True,
    url='http://github.com/containers/libpod',
    keywords='varlink libpod podman',
    classifiers=[
        'Development Status :: 3 - Alpha',
        'Intended Audience :: Developers',
        'License :: OSI Approved :: Apache Software License',
        'Programming Language :: Python :: 3.4',
        'Topic :: Software Development',
    ])
