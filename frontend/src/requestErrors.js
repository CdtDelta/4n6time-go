// A grid or histogram request cancelled by a newer one of the same kind
// rejects with this text (supersededText in request_slots.go). Callers drop it
// silently: no status bar error, no dialog.
export const isSuperseded = (err) => String(err).includes('request superseded by a newer request')
