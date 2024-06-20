import subprocess
from math import ceil
from os.path import basename, join, splitext
from time import sleep

from benchmark.commands import CommandMaker
from benchmark.config import Key, TSSKey, LocalCommittee, NodeParameters, BenchParameters, ConfigError
from benchmark.logs import LogParser, ParseError
from benchmark.utils import Print, BenchError, PathMaker
from datetime import datetime

class LocalBench:
    BASE_PORT = 6000

    def __init__(self, bench_parameters_dict, node_parameters_dict):
        try:
            self.ts = datetime.now().strftime(r"%Y-%m-%d-%H-%M-%S")
            self.bench_parameters = BenchParameters(bench_parameters_dict)
            self.node_parameters = NodeParameters(node_parameters_dict)
        except ConfigError as e:
            raise BenchError('Invalid nodes or bench parameters', e)


    def _background_run(self, command, log_file):
        name = splitext(basename(log_file))[0]
        cmd = f'{command} 2> {log_file}'
        subprocess.run(['tmux', 'new', '-d', '-s', name, cmd], check=True)

    def _kill_nodes(self):
        try:
            cmd = CommandMaker.kill().split()
            subprocess.run(cmd, stderr=subprocess.DEVNULL)
        except subprocess.SubprocessError as e:
            raise BenchError('Failed to kill testbed', e)

    def run(self, debug=False):
        assert isinstance(debug, bool)
        Print.heading('Starting local benchmark')

        # Kill any previous testbed.
        self._kill_nodes()

        try:
            Print.info('Setting up testbed...')
            nodes, rate,batch_size = self.bench_parameters.nodes[0], self.bench_parameters.rate,self.bench_parameters.batch_szie[0]
            self.node_parameters.json['pool']['rate'] = rate
            self.node_parameters.json['pool']['batch_size'] = batch_size 
            # Cleanup all files.
            cmd = f'{CommandMaker.cleanup_configs()} ; {CommandMaker.make_logs_and_result_dir(self.ts)}'
            subprocess.run([cmd], shell=True, stderr=subprocess.DEVNULL)
            sleep(0.5) # Removing the store may take time.

            # Recompile the latest code.
            cmd = CommandMaker.compile().split()
            subprocess.run(cmd, check=True)

            # Generate configuration files.
            keys = []
            key_files = [PathMaker.key_file(i) for i in range(nodes)]
            cmd = CommandMaker.generate_key(path="./",nodes=nodes).split()
            subprocess.run(cmd, check=True)
            for filename in key_files:
                keys += [Key.from_file(filename)]

            # Generate threshold signature files.
            tss_keys = []
            threshold_key_files = [PathMaker.threshold_key_file(i) for i in range(nodes)]
            N , T = nodes , 2 * (( nodes - 1 ) // 3) + 1
            cmd = CommandMaker.generate_tss_key(path = "./", N = N, T = T).split()
            subprocess.run(cmd, check=True)
            for filename in threshold_key_files:
                tss_keys += [TSSKey.from_file(filename)]

            # Generate committee file
            names = [x.pubkey for x in keys]
            ids = [i for i in range(nodes)]
            committee = LocalCommittee(names, ids, self.BASE_PORT)
            committee.print(PathMaker.committee_file())
            self.node_parameters.print(PathMaker.parameters_file())
            
            Print.info(f'Running {self.bench_parameters.protocol}')
            Print.info(f'{self.node_parameters.faults} byzantine nodes')
            Print.info(f'tx_size {self.node_parameters.tx_size} byte, batch_size {batch_size}, rate {rate} tx/s')
            Print.info(f'DDOS attack {self.node_parameters.ddos}')

            # Run the nodes.
            dbs = [PathMaker.db_path(i) for i in range(nodes)]
            node_logs = [PathMaker.node_log_error_file(i,self.ts) for i in range(nodes)]

            for id,key_file, threshold_key_file, db, log_file in zip(ids,key_files, threshold_key_files, dbs, node_logs):
                cmd = CommandMaker.run_node(
                    id,
                    key_file,
                    threshold_key_file,
                    PathMaker.committee_file(),
                    db,
                    PathMaker.parameters_file(),
                    self.ts,
                    self.bench_parameters.log_level
                )
                self._background_run(cmd, log_file)

            # Wait for the nodes to synchronize
            Print.info('Waiting for the nodes to synchronize...')
            sleep(2 * self.node_parameters.sync_timeout / 1000)

            # Wait for all transactions to be processed.
            Print.info(f'Running benchmark ({self.bench_parameters.duration} sec)...')
            sleep(self.bench_parameters.duration)
            self._kill_nodes()

            # Parse logs and return the parser.
            Print.info('Parsing logs...')
            return LogParser.process(
                PathMaker.logs_path(self.ts), 
                self.node_parameters.faults, 
                self.bench_parameters.protocol, 
                self.node_parameters.ddos
            )

        except (subprocess.SubprocessError, ParseError) as e:
            self._kill_nodes()
            raise BenchError('Failed to run benchmark', e)
