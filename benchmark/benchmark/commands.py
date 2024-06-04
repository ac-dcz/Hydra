from os.path import join

from benchmark.utils import PathMaker


class CommandMaker:

    @staticmethod
    def cleanup_configs():
        return f'rm -rf db-* ; rm -f .*.json'
        
    def cleanup_parameters():
        return f'rm -f .parameters.json'

    def cleanup_db():
        return f'rm -rf db-*'
    
    @staticmethod
    def make_logs_and_result_dir(ts):
        return f'mkdir -p {PathMaker.logs_path(ts)} ; mkdir -p {PathMaker.results_path(ts)}'

    @staticmethod
    def make_logs_dir(ts):
        return f'mkdir -p {PathMaker.logs_path(ts)}'
    
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
    def run_node(nodeid,keys, threshold_keys, committee, store, parameters, ts,level):
        assert isinstance(nodeid,int)
        assert isinstance(keys, str)
        assert isinstance(threshold_keys, str)
        assert isinstance(committee, str)
        assert isinstance(parameters, str)

        return (f'./main run --keys {keys} --threshold_keys {threshold_keys} --committee {committee} '
                f'--store {store} --parameters {parameters} --log_level {level} '
                f'--log_out {PathMaker.logs_path(ts)} --node_id {nodeid}')

    @staticmethod
    def kill():
        return 'tmux kill-server'
