import os

from setuptools import find_packages, setup


root = os.path.abspath(os.path.dirname(__file__))

with open(os.path.join(root, 'README.md')) as me:
    readme = me.read()

with open(os.path.join(root, 'requirements.txt')) as r:
    requirements = r.read().splitlines()

setup(
    name='podman',
    version='0.1.0',
    description='A client for communicating with a Podman server',
    long_description=readme,
    author='Jhon Honce',
    author_email='jhonce@redhat.com',
    url='http://github.com/projectatomic/libpod',
    license='Apache Software License',
    python_requires='>=3',
    include_package_data=True,
    install_requires=requirements,
    packages=find_packages(exclude=['test']),
    zip_safe=True,
    keywords='varlink libpod podman',
    classifiers=[
        'Development Status :: 3 - Alpha',
        'Intended Audience :: Developers',
        'Topic :: Software Development',
        'License :: OSI Approved :: Apache Software License',
        'Programming Language :: Python :: 3.6',
    ])
# Not supported
# long_description_content_type='text/markdown',
