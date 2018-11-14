Production Deployment Instructions
----------------------------------

1. Setup your storage account.
   1. Sign up for a Backblaze B2 account.
   2. Generate 8 bytes of randomness: `openssl rand -hex 8`
   3. Create a bucket. The bucket should be public and be named the random
      string you generated above; also set the random string as `b2_bucket` in
      your config.
   4. Click on "Lifecycle Settings", choose "Keep prior versions for 7 days",
      submit.
   5. Click "Show Account ID and Application Key". Save account ID as
      `b2_acct_id` in your config. Select "Create New Master Application Key"
      and save the application key as `b2_app_key`.
   6. Click on "Browse Files", click on your bucket, upload some file. Click on
      the file to bring up the info prompt. Take the "Friendly URL" and remove
      the filename from the end and the trailing slash. Save this URL as
      `b2_url` in your config.
2. Setup your DigitalOcean account.
   1. Create a droplet. Choose the recommended operating system (Ubuntu 16.04.4
       x64 at time of writing). Choose the smallest droplet offered: 1 GB of
       RAM, 25 GB SSD, 1 TB egress. Add 50 GB of block storage. Configure
       however else you want, and click create.
   2. Figure out where your volume is mounted with `df -h` while in the
      droplet. For me, it's `/mnt/volume_sfo2_02` so I set
      `bolt_path: /mnt/volume_sfo2_02/ct-log.db` in my config.
   3. Add a firewall to your account that allows SSH ingress from all IPv4 and
      IPv6, and restricts HTTPS ingress to Cloudflare's published IP ranges:
      https://www.cloudflare.com/ips/. Leave the defaults for egress. Make sure
      the firewall applies to the droplet your log is going to run in, and click
      create.
3. Setup your Cloudflare account.
   1. Move whatever domain you want to use to Cloudflare, if you haven't
      already. Go the the DNS tab. Add a AAAA record with the IPv6 address of
      your droplet.
   2. Create a Page Rule that covers your entire log, for example:
      `ct.cloudflare.com/*`. Set the SSL mode to 'Strict', enable 'Always Use
      HTTPS', enable 'Disable Security', and set 'Cache Level' to 'Cache
      Everything'. You may need multiple Page Rules.
   3. In the Crypto tab, generate an Origin Certificate, with an ECDSA keypair
      and the exact hostname that you're serving the CT log at. Leave the
      validity period at 15 years. Save the output certificate and private key
      to your droplet, and set the paths where they were saved as `cert_path`
      and `key_path` in your config.
   4. In the Workers tab, sign up for Workers if necessary. Create a new script
      called `get-entries`, where the script content is the same as
      devdata/get-entries.js, modified to correctly map path prefix to log ID.
      Save this and switch to the Routes tab of the editor and set any routes
      where you'll publish a get-entries endpoint to be served with the
      get-entries script. Note that the route needs to end in `*`, for example:
      `ct.cloudflare.com/logs/cirrus/ct/v1/get-entries*`.
   5. Back in the DNS tab, you can optionally create CNAME record for a random
      subdomain to the hostname of your `b2_url`. You can then switch the
      hostname of the `b2_url` with the random subdomain of your log, so that
      your backend also uses Cloudflare's edge to access B2.
4. Configure and start your CT log.
   1. Move all of your config and credentials over to the droplet:
      - Config file: based on `devdata/config.dev.yaml`, with
        `server_addr: 0.0.0.0:443` and the data you've collected from above.
      - Systemd unit: based on `devdata/ct-log.service`
      - Set of trusted roots: `devdata/certs.prod/ca-bundle.pem` is what Nimbus
        uses
   2. Load the unit file and try to start the service:
    ```
    # systemctl daemon-reload
    # systemctl start ct-log
    ```
   3. Check that the service is running and follow logs:
    ```
    # systemctl status ct-log
    # journalctl -f -u ct-log
    ```
    Note that you should be able to access the log on localhost:443 and through
    Cloudflare's edge, but not by directly dialing the droplet's address.
5. Setup metrics and alerts.
