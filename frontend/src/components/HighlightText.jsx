// HighlightText: wraps matched substrings in <mark> tags for keyword highlighting.
// Case-insensitive matching. Returns plain text if no search term is active.

function HighlightText({ text, search }) {
  if (!search || !text) return text || ''

  const str = String(text)
  const escaped = search.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${escaped})`, 'gi')
  const parts = str.split(regex)

  if (parts.length === 1) return str

  return parts.map((part, i) =>
    regex.test(part)
      ? <mark key={i} className="search-highlight">{part}</mark>
      : part
  )
}

export default HighlightText
