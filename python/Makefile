#export PYTHONASYNCIODEBUG=1
export PYTHONWARNINGS=default

test:
	python3 -m unittest

release:
	python3 setup.py register -r pypi
	python3 setup.py sdist upload -r pypi
