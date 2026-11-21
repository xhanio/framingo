// what builds the platform takes its handles
package platform

type Platform interface{ DB() any }

type bundle struct{ db any }

func (b *bundle) DB() any { return b.db }

func New(db any) Platform { return &bundle{db: db} }
