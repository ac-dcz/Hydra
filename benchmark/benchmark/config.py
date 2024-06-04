from json import dump, load


class ConfigError(Exception):
    pass


class Key:
    def __init__(self, pubkey, prikey):
        self.pubkey = pubkey
        self.prikey = prikey

    @classmethod
    def from_file(cls, filename):
        assert isinstance(filename, str)
        with open(filename, 'r') as f:
            data = load(f)
        return cls(data['public'], data['private'])

class TSSKey:
    def __init__(self, N, T,pub, share):
        self.N = N
        self.T = T
        self.pub = pub
        self.share = share

    @classmethod
    def from_file(cls, filename):
        assert isinstance(filename, str)
        with open(filename, 'r') as f:
            data = load(f)
        return cls(data['N'], data['T'],data['pub'], data['share'])


class Committee:
    def __init__(self, pubkeys, ids, consensus_addr):
        inputs = [pubkeys, consensus_addr]
        assert all(isinstance(x, list) for x in inputs)
        assert all(isinstance(x, str) for y in inputs for x in y)
        assert len({len(x) for x in inputs}) == 1

        self.pubkeys = pubkeys
        self.ids = ids
        self.consensus = consensus_addr

        self.json = self._build_consensus()

    def _build_consensus(self):
        node = {}
        for a, n, id in zip(self.consensus, self.pubkeys, self.ids):
            node[id] = {'name': n, 'addr': a, 'node_id': id}
        return node

    def print(self, filename):
        assert isinstance(filename, str)
        with open(filename, 'w') as f:
            dump(self.json, f, indent=4, sort_keys=True)

    def size(self):
        return len(self.json)


class LocalCommittee(Committee):
    def __init__(self, pubkeys, ids, port):
        assert isinstance(pubkeys, list) and all(isinstance(x, str) for x in pubkeys)
        assert isinstance(port, int)
        size = len(pubkeys)
        consensus = [f'127.0.0.1:{port + i}' for i in range(size)]
        super().__init__(pubkeys, ids, consensus)


class NodeParameters:
    def __init__(self, json):
        inputs = []
        try:
            inputs += [json['consensus']['sync_timeout']]
            inputs += [json['consensus']['network_delay']]
            inputs += [json['consensus']['min_block_delay']]
            inputs += [json['consensus']['ddos']]
            inputs += [json['consensus']['faults']]
            inputs += [json['consensus']['retry_delay']]
            inputs += [json['pool']['tx_size']]
            inputs += [json['pool']['max_queue_size']]
        except KeyError as e:
            raise ConfigError(f'Malformed parameters: missing key {e}')

        # if not all(isinstance(x, int) for x in inputs):
        #     raise ConfigError('Invalid parameters type')

        self.sync_timeout = json['consensus']['sync_timeout']
        self.network_delay = json['consensus']['network_delay'] 
        self.ddos = json['consensus']['ddos']
        self.faults = int(json['consensus']['faults'])
        self.tx_size = int(json['pool']['tx_size'])
        self.json = json

    def print(self, filename):
        assert isinstance(filename, str)
        with open(filename, 'w') as f:
            dump(self.json, f, indent=4, sort_keys=True)


class BenchParameters:
    def __init__(self, json):
        try:
            nodes = json['nodes'] 
            nodes = nodes if isinstance(nodes, list) else [nodes]
            if not nodes or any(x <= 0 for x in nodes):
                raise ConfigError('Missing or invalid number of nodes')
            
            batch_szie = json['batch_size'] 
            batch_szie = batch_szie if isinstance(batch_szie, list) else [batch_szie]
            if not batch_szie:
                raise ConfigError('Missing batch_size')

            self.nodes = [int(x) for x in nodes]
            self.log_level = int(json['log_level'])
            self.rate = int(json['rate'])
            self.batch_szie = [int(x) for x in batch_szie]
            self.duration = int(json['duration'])
            self.runs = int(json['runs']) if 'runs' in json else 1
            self.node_instance = int(json['node_instance']) if 'node_instance' in json else 1
            self.protocol = json['protocol_name']

        except KeyError as e:
            raise ConfigError(f'Malformed bench parameters: missing key {e}')

        except ValueError:
            raise ConfigError('Invalid parameters type')
