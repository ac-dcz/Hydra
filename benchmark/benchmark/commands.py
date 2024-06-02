from os.path import join

from benchmark.utils import PathMaker


class CommandMaker:

    @staticmethod
    def cleanup_configs():
        return (
            f'rm -r db-* ; rm *.json'
        )

    @staticmethod
    def make_logs_and_result_dir(ts):
        return f'mkdir -p {PathMaker.logs_path(ts)} ; mkdir -p {PathMaker.results_path(ts)}'

    @staticmethod
    def compile():
        return 'go build ../main.go'

    @staticmethod
    def generate_key(path,nodes):
        assert isinstance(path, str)
        return f'./main keys --path {path} --nodes {nodes}'
    
    @staticmethod
    def generate_tss_key(path,N,T):
        assert isinstance(path, str)
        return f'./main threshold_keys --path {path} --N {N} --T {T}'
    
    @staticmethod
    def run_node(nodeid,keys, threshold_keys, committee, store, parameters, ts,debug=False):
        assert isinstance(nodeid,int)
        assert isinstance(keys, str)
        assert isinstance(threshold_keys, str)
        assert isinstance(committee, str)
        assert isinstance(parameters, str)
        assert isinstance(debug, bool)
        level = 0x1 #info level
        if debug:
            level = 0xf #all level
        return (f'./main run --keys {keys} --threshold_keys {threshold_keys} --committee {committee} '
                f'--store {store} --parameters {parameters} --log_level {level} '
                f'--log_out {PathMaker.logs_path(ts)} --node_id {nodeid}')

    @staticmethod
    def kill():
        return 'tmux kill-server'

    @staticmethod
    def alias_binaries(origin):
        assert isinstance(origin, str)
        node, client = join(origin, 'node'), join(origin, 'client')
        return f'rm node ; rm client ; ln -s {node} . ; ln -s {client} .'
