# Rejection Rules

Rejection rules allow you to fine-tune your media downloads by automatically rejecting releases that you don't want. These rules are defined as regular expressions in your `config.yml` file.

### How it Works

When Reel searches for a download, it filters the results against your list of rejection rules. If a torrent's title matches any of the regular expressions, it will be rejected and not considered for download.

### Configuration

You can add your rejection rules to the `reject-common` section of your `config.yml` file:

```yaml
automation:
  reject-common:
    - \\bscreener\\b
    - \\bhdcam\\b
    - \\btelecine\\b
```

### Regular Expression Syntax

Here's a breakdown of the regular expression syntax used in the examples and how you can create your own rules:

* **`\b` (Word Boundary)**: This is one of the most useful operators for this use case. It matches the position between a word character (like a letter or number) and a non-word character (like a space, a period, or the start/end of the string). This is crucial for avoiding accidental matches.
    * **Example**: `\bhdcam\b` will match "HDCAM" but not "HDCAM-rip" or "anotherHDCAM". This ensures you're only matching the whole word.

* **`()` (Parentheses for Grouping)**: Parentheses are used to group parts of a regular expression together. This is useful for applying quantifiers to a whole group of characters or for capturing a part of the match.
    * **Example**: `(cam|screener)` will match either "cam" or "screener".

* **Delimiters**: In torrent names, words are often separated by periods (`.`), spaces, or hyphens (`-`). You can match these delimiters in your regex.
    * **Example**: To match "rus.dub", you could use `rus\.dub`. The backslash `\` is used to escape the period, which is a special character in regex.

* **Character Counts (Quantifiers)**: You can specify how many times a character or group of characters should appear.
    * `*`: Zero or more times
    * `+`: One or more times
    * `?`: Zero or one time
    * `{n}`: Exactly `n` times
    * `{n,}`: At least `n` times
    * `{n,m}`: Between `n` and `m` times
    * **Example**: `s\d{2}e\d{2}` will match "s01e01", "s12e22", etc.

### Examples
Here are some common examples of rejection rules:

- Reject screeners: `\bscreener\b`
- Reject CAM releases: `\bhdcam\b`
- Reject telecine releases: `\btelecine\b`
- Reject 3D releases: `\b3d\b`
- Reject Russian releases: `\brus\b`

You can add any regular expression to this list to customize your rejection rules.