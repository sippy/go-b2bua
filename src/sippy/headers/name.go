package sippy_header

type normalName struct {
    name    string
}

func newNormalName(name string) normalName {
    return normalName{
        name : name,
    }
}

func (self normalName) Name() string {
    return self.name
}

func (self normalName) CompactName() string {
    return self.name
}

type compactName struct {
    name            string
    compact_name    string
}

func newCompactName(name, compact_name string) compactName {
    return compactName{
        name         : name,
        compact_name : compact_name,
    }
}

func (self compactName) Name() string {
    return self.name
}

func (self compactName) CompactName() string {
    return self.compact_name
}
