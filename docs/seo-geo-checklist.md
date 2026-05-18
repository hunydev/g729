# SEO/GEO Checklist

This checklist records repository and GitHub Pages discoverability tasks for
`github.com/hunydev/g729` and <https://g729.huny.dev/>. It is an engineering
checklist, not a promise of search ranking or AI citation.

## Google Generative AI Search Reality Check

- [ ] Treat AEO/GEO work as normal SEO for Google Search, not as a separate
      hack layer.
- [ ] Prioritize helpful, unique, people-first content over AI-specific files.
- [ ] Use the project's non-commodity evidence: clean-room provenance, decoder
      exact validation, RTP scope, performance metrics, and listening samples.
- [ ] Do not create mass query-variant pages solely to capture fan-out queries.
- [ ] Do not rewrite pages only for AI systems or stuff long-tail keyword
      variants into copy.
- [ ] Do not treat `llms.txt` or Markdown summaries as required Google Search
      markup.
- [ ] Use structured data only when it is accurate and useful for normal SEO.

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
- [ ] `docs/llms.txt` exists as an optional informal orientation file, not a
      Google ranking requirement.
- [ ] `docs/ai-summary.md` exists as a quote-safe canonical facts page.
- [ ] FAQ and canonical facts are easy to read as static HTML or Markdown.
- [ ] No page claims ITU certification, ITU endorsement, encoder byte-exact
      conformance, Annex B support, G.729.1 support, or G.729D/E support.

## Agent-Friendly And Accessibility Checks

- [ ] Important project identity, claims, and links are visible in the DOM.
- [ ] Interactive audio controls have clear labels.
- [ ] Canvas waveform visuals are decorative or backed by text labels.
- [ ] The FAQ, validation summary, and clean-room links are usable without
      running the codec WebAssembly demo.
- [ ] Page layout is usable on desktop and mobile without horizontal overflow.

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

## Not Applicable For This Project

- Google Business Profile is not applicable unless a separate business listing
  is intentionally maintained.
- Google Merchant Center is not applicable unless paid products or commercial
  listings are added.

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

`llms.txt` is an informal proposal and should be treated as a low-cost optional
orientation file, not a guaranteed AI ranking or citation mechanism. Google's
generative AI Search features rely on normal Search indexing and quality
systems, so the canonical evidence remains the repository README, validation
docs, claim boundary docs, and clean-room provenance records.
