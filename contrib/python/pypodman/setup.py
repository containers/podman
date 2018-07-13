import os

from setuptools import find_packages, setup

root = os.path.abspath(os.path.dirname(__file__))

with open(os.path.join(root, 'README.md')) as me:
    readme = me.read()

with open(os.path.join(root, 'requirements.txt')) as r:
    requirements = r.read().splitlines()

setup(
    name='pypodman',
    version=os.environ.get('PODMAN_VERSION', '0.0.0'),
    description='A client for communicating with a Podman server',
    author_email='jhonce@redhat.com',
    author='Jhon Honce',
    license='Apache Software License',
    long_description=readme,
    entry_points={'console_scripts': [
        'pypodman = lib.pypodman:main',
    ]},
    include_package_data=True,
    install_requires=requirements,
    keywords='varlink libpod podman pypodman',
    packages=find_packages(exclude=['test']),
    python_requires='>=3',
    zip_safe=True,
    url='http://github.com/projectatomic/libpod',
    classifiers=[
        'Development Status :: 3 - Alpha',
        'Intended Audience :: Developers',
        'Intended Audience :: System Administrators',
        'License :: OSI Approved :: Apache Software License',
        'Operating System :: MacOS :: MacOS X',
        'Operating System :: Microsoft :: Windows',
        'Operating System :: POSIX',
        'Programming Language :: Python :: 3.6',
        'Topic :: System :: Systems Administration',
        'Topic :: Utilities',
    ])
