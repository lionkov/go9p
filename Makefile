# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# After editing the DIRS= list or adding imports to any Go files
# in any of those directories, run:
#
#   ./deps.bash
#
# to rebuild the dependency information in Make.deps.

all: install 

DIRS=\
	p\
    p/clnt\
    p/srv\

NOTEST=\
	p\
    p/clnt\
    p/srv\

EXAMPLES=\
    p/clnt/examples\
	p/srv/examples\

clean.dirs: $(addsuffix .clean, $(DIRS))
clean.dirs: $(addsuffix .clean, $(EXAMPLES))
install.dirs: $(addsuffix .install, $(DIRS))
nuke.dirs: $(addsuffix .nuke, $(DIRS))
examples.dirs: $(addsuffix .examples, $(EXAMPLES))

%.clean:
	+cd $* && gomake clean

%.install:
	+cd $* && gomake install

%.nuke:
	+cd $* && gomake nuke

%.test:
	+cd $* && gomake test

%.examples:
	+cd $* && gomake

clean: clean.dirs

install: $(TARG) install.dirs

nuke: nuke.dirs

examples: examples.dirs


