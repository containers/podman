.PHONY: dist
dist: crio.spec
	spectool -g crio.spec

.PHONY: rpm
rpm: dist
	rpmbuild --define "_sourcedir `pwd`" --define "_specdir `pwd`" \
	--define "_rpmdir `pwd`" --define "_srcrpmdir `pwd`" -ba crio.spec

all: rpm

clean:
	rm -f *rpm *gz
	rm -rf x86_64
