LATEXTOPDF=pdflatex
PROGRAM=Keylime_MCP_Inception
MV=mv

all: $(PROGRAM).pdf

$(PROGRAM).pdf:
	$(LATEXTOPDF) 000-keylime-mcp.tex
	$(LATEXTOPDF) 000-keylime-mcp.tex
	$(MV) 000-keylime-mcp.pdf $(PROGRAM).pdf
clean:
	rm -frv *.aux *.nav *.out *.toc *.log *.snm *.vrb *~
clobber:
	rm -frv *.aux *.nav *.out *.toc *.pdf *.log *.snm *.vrb *~
test:
	@-true

.PHONY: all $(PROGRAM).pdf clean clobber test
