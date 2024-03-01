# v3.0.2

## Fixed

 - DecryptMulti: handle decompression error (#19)

## Changed

 - jwe/CompactSerialize: improve performance (#67)
 - Increase the default number of PBKDF2 iterations to 600k (#48)
 - Return the proper algorithm for ECDSA keys (#45)

## Added

 - Add Thumbprint support for opaque signers (#38)

# v3.0.1

## Fixed

 - Security issue: an attacker specifying a large "p2c" value can cause
   JSONWebEncryption.Decrypt and JSONWebEncryption.DecryptMulti to consume large
   amounts of CPU, causing a DoS. Thanks to Matt Schwager (@mschwager) for the
   disclosure and to Tom Tervoort for originally publishing the category of attack.
   https://i.blackhat.com/BH-US-23/Presentations/US-23-Tervoort-Three-New-Attacks-Against-JSON-Web-Tokens.pdf
