from datetime import datetime
from os import error
from fabric import Connection, ThreadingGroup as Group
from fabric.exceptions import GroupException
from paramiko import RSAKey
from paramiko.ssh_exception import PasswordRequiredException, SSHException
from os.path import basename, splitext
from time import sleep
from math import ceil
from os.path import join
import subprocess

from benchmark.config import Committee, Key, TSSKey, NodeParameters, BenchParameters, ConfigError
from benchmark.utils import BenchError, Print, PathMaker, progress_bar
from benchmark.commands import CommandMaker
from benchmark.logs import LogParser, ParseError
from alibaba.instance import InstanceManager


class FabricError(Exception):
    ''' Wrapper for Fabric exception with a meaningfull error message. '''

    def __init__(self, error):
        assert isinstance(error, GroupException)
        message = list(error.result.values())[-1]
        super().__init__(message)


class ExecutionError(Exception):
    pass


class Bench:
    def __init__(self, ctx):
        self.manager = InstanceManager.make()
        self.settings = self.manager.settings
        try:
            # ssh 连接
            ctx.connect_kwargs.pkey = RSAKey.from_private_key_file(
                self.manager.settings.key_path
            )
            self.connect = ctx.connect_kwargs
        except (IOError, PasswordRequiredException, SSHException) as e:
            raise BenchError('Failed to load SSH key', e)

    def _check_stderr(self, output):
        if isinstance(output, dict):
            for x in output.values():
                if x.stderr:
                    raise ExecutionError(x.stderr)
        else:
            if output.stderr:
                raise ExecutionError(output.stderr)

    def kill(self, hosts=[], delete_logs=False):
        assert isinstance(hosts, list)
        assert isinstance(delete_logs, bool)
        hosts = hosts if hosts else self.manager.hosts(flat=True)

        cmd = ["true", f'({CommandMaker.kill()} || true)']

        # note: please set hostname (ubuntu) 
        try:
            g = Group(*hosts, user='root', connect_kwargs=self.connect)
            g.run(' && '.join(cmd), hide=True)
        except GroupException as e:
            raise BenchError('Failed to kill nodes', FabricError(e))

    def _select_hosts(self, bench_parameters):
        nodes = max(bench_parameters.nodes)

        # Ensure there are enough hosts.
        hosts = self.manager.hosts()
        if sum(len(x) for x in hosts.values()) < nodes:
            return []

        # Select the hosts in different data centers.
        ordered = [x for y in hosts.values() for x in y]
        return ordered[:nodes]

    def _background_run(self, host, command, log_file):
        name = splitext(basename(log_file))[0]
        cmd = f'tmux new -d -s "{name}" "{command} |& tee {log_file}"'
        c = Connection(host, user='root', connect_kwargs=self.connect)
        output = c.run(cmd, hide=True)
        self._check_stderr(output)

    def _update(self, hosts,node_parameters,ts):

        # update parameters file
        Print.info(
            f'Updating {len(hosts)} nodes ...'
        )
        
        # Cleanup all local configuration files.
        cmd = CommandMaker.cleanup_parameters()
        subprocess.run([cmd], shell=True, stderr=subprocess.DEVNULL)

        # 
        cmd = CommandMaker.make_logs_and_result_dir(ts)
        subprocess.run([cmd], shell=True, stderr=subprocess.DEVNULL)

        # Cleanup all nodes.
        cmd = [CommandMaker.cleanup_parameters(),CommandMaker.cleanup_db(),CommandMaker.make_logs_dir(ts)]
        g = Group(*hosts, user='root', connect_kwargs=self.connect)
        g.run("&&".join(cmd), hide=True)

        node_parameters.print(PathMaker.parameters_file()) #generate new parameters
        # Update configuration files.
        progress = progress_bar(hosts, prefix='Update parameters files:')
        for host in progress:
            c = Connection(host, user='root', connect_kwargs=self.connect)
            c.put(PathMaker.parameters_file(), '.')

    def install(self):

        cmd = [
            'sudo apt-get update',
            'sudo apt-get -y upgrade',
            'sudo apt-get -y autoremove',

            # The following dependencies prevent the error: [error: linker `cc` not found].
            'sudo apt-get -y install tmux',
        ]
       
        hosts = self.manager.hosts(flat=True)
        try:
            g = Group(*hosts, user='root', connect_kwargs=self.connect)
            g.run(' && '.join(cmd), hide=True)
            Print.heading(f'Initialized testbed of {len(hosts)} nodes')
        except (GroupException, ExecutionError) as e:
            e = FabricError(e) if isinstance(e, GroupException) else e
            raise BenchError('Failed to install repo on testbed', e)

    def upload_exec(self):
        hosts = self.manager.hosts(flat=True)
        # Recompile the latest code.
        cmd = CommandMaker.compile().split()
        subprocess.run(cmd, check=True)
        # Upload execute files.
        progress = progress_bar(hosts, prefix='Uploading main files:')
        for host in progress:
            c = Connection(host, user='root', connect_kwargs=self.connect)
            c.put(PathMaker.execute_file(),'.')

    def _config(self, hosts,bench_parameters):
        Print.info('Generating configuration files...')

        # Cleanup all local configuration files.
        cmd = CommandMaker.cleanup_configs()
        subprocess.run([cmd], shell=True, stderr=subprocess.DEVNULL)

        node_instance = bench_parameters.node_instance
        nodes = len(hosts) * node_instance
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

        names = [x.pubkey for x in keys]
        ids = [i for i in range(nodes)]
        consensus_addr = []
        for ip in hosts:
            for i in range(node_instance):
                consensus_addr += [f'{ip}:{self.settings.consensus_port+i}']

        committee = Committee(names, ids, consensus_addr)
        committee.print(PathMaker.committee_file())

        # Cleanup all nodes.
        cmd = f'{CommandMaker.cleanup_configs()} || true'
        g = Group(*hosts, user='root', connect_kwargs=self.connect)
        g.run(cmd, hide=True)

        # Upload configuration files.
        progress = progress_bar(hosts, prefix='Uploading config files:')
        for i, host in enumerate(progress):
            c = Connection(host, user='root', connect_kwargs=self.connect)
            c.put(PathMaker.committee_file(), '.')
            for j in range(node_instance):
                c.put(PathMaker.key_file(i*node_instance+j), '.')
                c.put(PathMaker.threshold_key_file(i*node_instance+j), '.')

        return committee

    def _run_single(self, hosts, bench_parameters, ts, debug=False):
        Print.info('Booting testbed...')

        # Kill any potentially unfinished run and delete logs.
        self.kill(hosts=hosts, delete_logs=True)
        Print.info('Killed previous instances')
        sleep(10)

        node_instance = bench_parameters.node_instance
        nodes = len(hosts) * node_instance

        # Run the nodes.
        key_files = [PathMaker.key_file(i) for i in range(nodes)]
        threshold_key_files = [PathMaker.threshold_key_file(i) for i in range(nodes)]
        dbs = [PathMaker.db_path(i) for i in range(nodes)]
        node_logs = [PathMaker.node_log_error_file(i,ts) for i in range(nodes)]
        for i,host in enumerate(hosts):
            for j in range(node_instance):
                cmd = CommandMaker.run_node(
                    i*node_instance+j,
                    key_files[i*node_instance+j],
                    threshold_key_files[i*node_instance+j],
                    PathMaker.committee_file(),
                    dbs[i*node_instance+j],
                    PathMaker.parameters_file(),
                    ts,
                    bench_parameters.log_level
                )
                self._background_run(host, cmd, node_logs[i*node_instance+j])

        # Wait for the nodes to synchronize
        Print.info('Waiting for the nodes to synchronize...')
        sleep(20)

        # Wait for all transactions to be processed.
        duration = bench_parameters.duration
        for _ in progress_bar(range(100), prefix=f'Running benchmark ({duration} sec):'):
            sleep(ceil(duration / 100))
        self.kill(hosts=hosts, delete_logs=False)

    def download(self,node_instance,ts):
        hosts = self.manager.hosts(flat=True)
        # Download log files.
        progress = progress_bar(hosts, prefix='Downloading logs:')
        for i, host in enumerate(progress):
            c = Connection(host, user='root', connect_kwargs=self.connect)
            for j in range(node_instance):
                # c.get(PathMaker.node_log_info_file(i*node_instance+j,ts), local=PathMaker.node_log_info_file(i*node_instance+j,ts))
                c.get(PathMaker.node_log_debug_file(i*node_instance+j,ts), local=PathMaker.node_log_debug_file(i*node_instance+j,ts))
                # c.get(PathMaker.node_log_error_file(i*node_instance+j,ts), local=PathMaker.node_log_error_file(i*node_instance+j,ts))
                # c.get(PathMaker.node_log_warn_file(i*node_instance+j,ts), local=PathMaker.node_log_warn_file(i*node_instance+j,ts))

        # Parse logs and return the parser.
        Print.info('Parsing logs and computing performance...')
        return LogParser.process(PathMaker.logs_path(ts))
    
    def _logs(self, hosts, faults, protocol, ddos,bench_parameters,ts):
        
        node_instance = bench_parameters.node_instance
        # Download log files.
        progress = progress_bar(hosts, prefix='Downloading logs:')
        for i, host in enumerate(progress):
            c = Connection(host, user='root', connect_kwargs=self.connect)
            for j in range(node_instance):
                c.get(PathMaker.node_log_info_file(i*node_instance+j,ts), local=PathMaker.node_log_info_file(i*node_instance+j,ts))
                # c.get(PathMaker.node_log_debug_file(i*node_instance+j,ts), local=PathMaker.node_log_debug_file(i*node_instance+j,ts))
                # c.get(PathMaker.node_log_error_file(i*node_instance+j,ts), local=PathMaker.node_log_error_file(i*node_instance+j,ts))
                # c.get(PathMaker.node_log_warn_file(i*node_instance+j,ts), local=PathMaker.node_log_warn_file(i*node_instance+j,ts))

        # Parse logs and return the parser.
        Print.info('Parsing logs and computing performance...')
        return LogParser.process(PathMaker.logs_path(ts), faults=faults, protocol=protocol, ddos=ddos)

    def run(self, bench_parameters_dict, node_parameters_dict, debug=False):
        assert isinstance(debug, bool)

        Print.heading('Starting remote benchmark')
        try:
            bench_parameters = BenchParameters(bench_parameters_dict)
            node_parameters = NodeParameters(node_parameters_dict)
        except ConfigError as e:
            raise BenchError('Invalid nodes or bench parameters', e)

        # Recompile the latest code.
        cmd = CommandMaker.compile().split()
        subprocess.run(cmd, check=True)

        #Step 1: Select which hosts to use.
        selected_hosts = self._select_hosts(bench_parameters)
        if not selected_hosts:
            Print.warn('There are not enough instances available')
            return

        Print.info(f'Running {bench_parameters.protocol}')
        Print.info(f'{node_parameters.faults} byzantine nodes')
        Print.info(f'tx_size {node_parameters.tx_size} byte, input rate {bench_parameters.rate} tx/s')
        Print.info(f'DDOS attack {node_parameters.ddos}')
        
        #Step 2: Run benchmarks.
        for n in bench_parameters.nodes:
            
            hosts = selected_hosts[:n]
            #Step 3: Upload all configuration files.
            try:
                self._config(hosts, bench_parameters)
            except (subprocess.SubprocessError, GroupException) as e:
                e = FabricError(e) if isinstance(e, GroupException) else e
                Print.error(BenchError('Failed to configure nodes', e))

            for batch_size in bench_parameters.batch_szie:
                Print.heading(f'\nRunning {n}/{bench_parameters.node_instance} nodes (batch size: {batch_size:,})')
                hosts = selected_hosts[:n]

                node_parameters.json['pool']['rate'] = bench_parameters.rate
                node_parameters.json['pool']['batch_size'] = batch_size
                self.ts = datetime.now().strftime(r"%Y-%m-%d-%H-%M-%S")
                
                #Step a: only upload parameters files.
                try:
                    self._update(hosts,node_parameters,self.ts)
                except (subprocess.SubprocessError, GroupException) as e:
                    e = FabricError(e) if isinstance(e, GroupException) else e
                    Print.error(BenchError('Failed to update nodes', e))
                    continue

                protocol = bench_parameters.protocol
                ddos = node_parameters.ddos

                # Run the benchmark.
                for i in range(bench_parameters.runs):
                    Print.heading(f'Run {i+1}/{bench_parameters.runs}')
                    try:
                        self._run_single(
                            hosts,bench_parameters, self.ts , debug
                        )
                        self._logs(hosts, node_parameters.faults , protocol, ddos,bench_parameters,self.ts).print(
                            PathMaker.result_file(
                                n, bench_parameters.rate, node_parameters.tx_size,batch_size , node_parameters.faults,self.ts
                        ))
                    except (subprocess.SubprocessError, GroupException, ParseError) as e:
                        self.kill(hosts=hosts)
                        if isinstance(e, GroupException):
                            e = FabricError(e)
                        Print.error(BenchError('Benchmark failed', e))
                        continue
