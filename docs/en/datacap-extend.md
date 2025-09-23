# Datacap extend

The purpose of **datacap extend** is to adjust the maximum term of a datacap, but the maximum term cannot exceed **5 years (5,256,000 epochs)**.

## Extend Commands

### 1. Manually specify claim ID, set a new maximum term, and storage provider

```
# --from must be a datacap address; if omitted, the droplet client’s default address is used
# --max-term the new maximum term
./droplet-client datacap extend --max-term 1909497 --from <address> --claimId 1 <address>

eg.
./droplet-client datacap extend --max-term 1909497 --from t3wp7bkktkeybm42kvxtyuqsmod262fmwn7zuo3nf3xll34xaokhm4n4x5rgivwg6fcu2mnjecourodjmil3fq --claimId 1 --claimId 2 t01000
```

You can view the claims of a storage provider with:

```
./droplet-client datacap list-claim <address>

eg.
./droplet-client datacap list-claim t01000
```

### 2. Automatically select eligible datacap

```
# --from must be a datacap address; if omitted, the droplet client’s default address is used
# --max-term the new maximum term
# --expiration-cutoff ignore datacap with expiration later than the cutoff.
#   Example: if cutoff is 2880 (one day), only datacap expiring in less than one day will be renewed.
# --max-claims the number of claims included in each renewal message
./droplet-client datacap extend --max-claims 310 --max-term 1909697 --from <address> --auto --expiration-cutoff 2880 <address>

eg.
./droplet-client datacap extend --max-claims 310 --max-term 1909597 --from t3wp7bkktkeybm42kvxtyuqsmod262fmwn7zuo3nf3xll34xaokhm4n4x5rgivwg6fcu2mnjecourodjmil3fq --auto --expiration-cutoff 2880 t01000
```
