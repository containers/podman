# License Classifier v2

This is a substantial revision of the license classifier with a focus on improved accuracy and performance.

## Glossary

- corpus dictionary - contains all the unique tokens stored in the corpus of
documents to match. Any tokens in the target document that aren't in the corpus
dictionary are mapped to an invalid value.

- document - an internal-only data type that contains sequenced token information
for a source or target content for matching.

- source content - a body of text that can be matched by the scanner.

- target content - the argument to Match that is scanned for matches with source
content.

- indexed document - an internal-only data type that maps a document to the
corpus dictionary, resulting in a compressed representation suitable for fast
text searching and mapping operations. an indexed document is necessarily
tightly coupled to its corpus.

- frequency table - a lookup table holding per-token counts of the number of
times a token appears in content. used for fast filtering of target content
against different source contents.

- q-gram - a substring of content of length q tokens used to efficiently match
ranges of text. For background on the q-gram algorithms used, please see
[Indexing Methods for Approximate String Matching](https://users.dcc.uchile.cl/~gnavarro/ps/deb01.pdf)

- searchset - a data structure that uses q-grams to identify ranges of text in
the target that correspond to a range of text in the source. The searchset
algorithms compensate for the allowable error in matching text exactly, dealing
with additional or missing tokens.


## Migrating from v1

The API for the classifier versions is quite similar, but there are two key
distinctions to be aware of while migrating usages.

The confidence value for the v2 classifier is applied uniformly to results; it
will never return a match that is lower confidence than the threshold. In v1,
MultipleMatch behaved this way, but NearestMatch would return a value
regardless of the confidence match. Users often verified that the confidence
was above the threshold, but this is no longer necessary.

The second change is that the classifier now returns all matches against the
supplied corpus. The v1 classifier allowed filtering on header matches via a
boolean field. This can be emulated by creating a license classifier with a
reduced corpus if matching against headers is not desired. Alternatively, the
user can use the MatchType field in the Match struct to filter out unwanted
matches.


