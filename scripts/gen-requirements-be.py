OUTPUT = './docs/requirements-be.md'


class Module:
    def __init__(self, line):
        split = line.split(' ')
        self.url = split[0]
        self.version = split[1]
        self.name = '/'.join(self.url.split('/')[-2:])

        urlsplit = self.url.split('/')
        if urlsplit[-1].startswith('v') and urlsplit[-1][1].isdigit():
            self.url = '/'.join(urlsplit[:-1])

    def string(self):
        return '[{}](https://{}) `({})`'.format(self.name, self.url, self.version)


def main():
    lines = []
    with open('./go.mod') as f:
        lines = [l.strip() for l in f.readlines()]
    start = lines.index('require (')
    end = lines.index(')')
    lines = [l for l in lines[start+1:end] if not l.endswith('// indirect')]
    modules = [Module(l) for l in lines]

    with open(OUTPUT, 'w') as f:
        f.write('<!-- insert:REQUIREMENTS_BE -->\n')
        f.writelines(['- {}\n'.format(m.string()) for m in modules])


if __name__ == '__main__':
    main()
