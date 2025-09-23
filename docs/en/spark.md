## Spark

### Enable

1. Open the `droplet` configuration file `config.toml` and add the following under `[CommonProvider]`.
   The `PublicHostname` should be your public IP address.

```toml
  [CommonProvider.IndexProvider]
    Enable = true
    EntriesCacheCapacity = 1024
    EntriesChunkSize = 16384
    TopicName = ""
    PurgeCacheOnStart = false
    WebHost = "cid.contact"
    DataTransferPublisher = true
    [CommonProvider.IndexProvider.Announce]
      AnnounceOverHttp = true
      DirectAnnounceURLs = ["https://cid.contact/ingest/announce"]
    [CommonProvider.IndexProvider.HttpPublisher]
      Enabled = true
      PublicHostname = "127.0.0.1"
      Port = 41263
      WithLibp2p = true
```

2. Adjust the libp2p configuration:

```toml
[Libp2p]
  ListenAddresses = ["/ip4/0.0.0.0/tcp/58418"]
  AnnounceAddresses = ["/ip4/<YOUR_PUBLIC_IP_ADDRESS>/tcp/58418"]
```

---

### CLI Commands

1. Announce all deals of a single miner to IPNI:

```bash
./droplet index announce-all --miner t01001
```

2. Announce a single deal to IPNI:

```bash
./droplet index announce-deal <deal uuid> 
# or
./droplet index announce-deal <proposal cid>
```

3. Remove a deal from IPNI:

```bash
./droplet index announce-remove-deal <deal uuid> 
# or
./droplet index announce-remove-deal <proposal cid>
```

### Verification

1. Visit [https://cid.contact/](https://cid.contact/) and enter the deal’s `payload cid` to check if it has been published to IPNI.
2. Check whether the miner peer ID has been registered on IPNI at:
   [https://cid.contact/providers/12D3KooWLR8GZNKpN7zM6T24zw1z4gQrCdk2tFWzG8mfFaaszGay](https://cid.contact/providers/12D3KooWLR8GZNKpN7zM6T24zw1z4gQrCdk2tFWzG8mfFaaszGay)

### Notes

* `droplet` version must be at least **v2.12.0**
* `venus` version must be at least **v1.16.0**
