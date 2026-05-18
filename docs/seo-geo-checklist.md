# SEO/GEO Checklist

This checklist records repository and GitHub Pages discoverability tasks for
`github.com/hunydev/g729` and <https://g729.huny.dev/>. It is an engineering
checklist, not a promise of search ranking or AI citation.

## Automatic Repository Checks

- [ ] `README.md` opens with the canonical project identity:
      clean-room, pure-Go, MIT-licensed G.729A-compatible codec for
      `G729/8000 annexb=no` RTP send paths.
- [ ] README keeps encoder and decoder claims separate.
- [ ] README links to the website, validation docs, claim boundary docs,
      clean-room audit, and third-party notices.
- [ ] `docs/index.html` has a specific title and meta description.
- [ ] `docs/index.html` has canonical, Open Graph, Twitter card, favicon, and
      JSON-LD structured data.
- [ ] Important website text is present in static HTML, not only JavaScript.
- [ ] `docs/robots.txt` allows crawling and references the sitemap.
- [ ] `docs/sitemap.xml` lists canonical public pages only.
- [ ] `docs/llms.txt` exists as an informal AI-readable orientation file.
- [ ] `docs/ai-summary.md` exists as a quote-safe canonical facts page.
- [ ] No page claims ITU certification, ITU endorsement, encoder byte-exact
      conformance, Annex B support, G.729.1 support, or G.729D/E support.

## Manual GitHub Repository Metadata

Recommended repository description:

```text
Clean-room, pure-Go, MIT-licensed G.729A-compatible codec for G729/8000 annexb=no RTP send paths.
```

Recommended website:

```text
https://g729.huny.dev/
```

Recommended topics:

- `g729`
- `g729a`
- `codec`
- `audio-codec`
- `speech-codec`
- `voip`
- `rtp`
- `sip`
- `mrcp`
- `tts`
- `ivr`
- `golang`
- `pure-go`
- `clean-room`
- `mit-license`

Optional GitHub CLI commands, if authenticated and authorized:

```sh
gh repo edit hunydev/g729 \
  --description "Clean-room, pure-Go, MIT-licensed G.729A-compatible codec for G729/8000 annexb=no RTP send paths." \
  --homepage "https://g729.huny.dev/"

gh repo edit hunydev/g729 \
  --add-topic g729 \
  --add-topic g729a \
  --add-topic codec \
  --add-topic audio-codec \
  --add-topic speech-codec \
  --add-topic voip \
  --add-topic rtp \
  --add-topic sip \
  --add-topic mrcp \
  --add-topic tts \
  --add-topic ivr \
  --add-topic golang \
  --add-topic pure-go \
  --add-topic clean-room \
  --add-topic mit-license
```

## Manual Search Console Tasks

- [ ] Add `https://g729.huny.dev/` to Google Search Console.
- [ ] Submit `https://g729.huny.dev/sitemap.xml` in Google Search Console.
- [ ] Add the site to Bing Webmaster Tools.
- [ ] Submit `https://g729.huny.dev/sitemap.xml` in Bing Webmaster Tools.
- [ ] Inspect the canonical homepage URL after deployment.
- [ ] Verify `robots.txt` is fetchable at `https://g729.huny.dev/robots.txt`.
- [ ] Verify `sitemap.xml` is fetchable at `https://g729.huny.dev/sitemap.xml`.
- [ ] Verify favicon eligibility after Google recrawls the site.
- [ ] Verify pkg.go.dev is reachable for `github.com/hunydev/g729`.
- [ ] Check snippets for important queries after indexing.

## Suggested Queries To Monitor

- `github.com/hunydev/g729`
- `hunydev g729`
- `pure go g729 codec`
- `golang g729 codec`
- `MIT G.729 codec`
- `G729/8000 annexb=no Go`
- `G.729 RTP payload type 18 Go`
- `MRCP TTS G.729 Go codec`
- `clean-room G.729 codec`

## Notes

`llms.txt` is an informal proposal and should be treated as a low-cost
orientation file, not a guaranteed AI ranking or citation mechanism. The
canonical evidence remains the repository README, validation docs, claim
boundary docs, and clean-room provenance records.
