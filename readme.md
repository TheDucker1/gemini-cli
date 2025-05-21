# Vibe Debugging with Google Gemini (Pro 2.5 May Preview)

Google Gemini just add a new functionality: url context, which would read url content without taking 500K - 1M tokens for a rather more-complex-than-just-a-simple-page.

So I use that function, throwing in 20 pages from Gemini docs, and tell it to convert the HTML content to one big Markdown file as a docs guide with the flash model. 

Throwing that docs guide into the pro model, we have this program. This only use the REST api, and only support text output.

This is a toy program, I have NOT extensively tested it, used at your own risk.

For example, the api token is stored in your computer in a plaintext json file, so anyone could read/get it.

Installing:

```
git clone https://github.com/TheDucker1/gemini-cli.git
cd gemini-cli
go build
```