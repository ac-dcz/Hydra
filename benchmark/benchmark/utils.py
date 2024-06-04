from os.path import join


class BenchError(Exception):
    def __init__(self, message, error):
        assert isinstance(error, Exception)
        self.message = message
        self.cause = error
        super().__init__(message)


class PathMaker:
    @staticmethod
    def execute_file():
        return "main"
    
    @staticmethod
    def committee_file():
        return '.committee.json'

    @staticmethod
    def parameters_file():
        return '.parameters.json'

    @staticmethod
    def key_file(i):
        assert isinstance(i, int) and i >= 0
        return f'.node-key-{i}.json'

    @staticmethod
    def threshold_key_file(i):
        assert isinstance(i, int) and i >= 0
        return f'.node-ts-key-{i}.json'
        
    @staticmethod
    def db_path(i):
        assert isinstance(i, int) and i >= 0
        return f'db-{i}'

    @staticmethod
    def logs_path(ts):
        assert isinstance(ts, str)
        return f'logs/{ts}'

    @staticmethod
    def node_log_info_file(i,ts):
        assert isinstance(i, int) and i >= 0
        return join(PathMaker.logs_path(ts), f'node-info-{i}.log')
    
    @staticmethod
    def node_log_debug_file(i,ts):
        assert isinstance(i, int) and i >= 0
        return join(PathMaker.logs_path(ts), f'node-debug-{i}.log')
    
    @staticmethod
    def node_log_warn_file(i,ts):
        assert isinstance(i, int) and i >= 0
        return join(PathMaker.logs_path(ts), f'node-warn-{i}.log')
    
    @staticmethod
    def node_log_error_file(i,ts):
        assert isinstance(i, int) and i >= 0
        return join(PathMaker.logs_path(ts), f'node-error-{i}.log')

    @staticmethod
    def results_path(ts):
        assert isinstance(ts, str)
        return f'results/{ts}'

    @staticmethod
    def result_file(nodes, rate, tx_size, batch_size ,faults,ts):
        return join(
            PathMaker.results_path(ts), f'bench-{nodes}-{rate}-{tx_size}-{batch_size}-{faults}.txt'
        )


class Color:
    HEADER = '\033[95m'
    OK_BLUE = '\033[94m'
    OK_GREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    END = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'


class Print:
    @staticmethod
    def heading(message):
        assert isinstance(message, str)
        print(f'{Color.OK_GREEN}{message}{Color.END}')

    @staticmethod
    def info(message):
        assert isinstance(message, str)
        print(message)

    @staticmethod
    def warn(message):
        assert isinstance(message, str)
        print(f'{Color.BOLD}{Color.WARNING}WARN{Color.END}: {message}')

    @staticmethod
    def error(e):
        assert isinstance(e, BenchError)
        print(f'\n{Color.BOLD}{Color.FAIL}ERROR{Color.END}: {e}\n')
        causes, current_cause = [], e.cause
        while isinstance(current_cause, BenchError):
            causes += [f'  {len(causes)}: {e.cause}\n']
            current_cause = current_cause.cause
        causes += [f'  {len(causes)}: {type(current_cause)}\n']
        causes += [f'  {len(causes)}: {current_cause}\n']
        print(f'Caused by: \n{"".join(causes)}\n')


def progress_bar(iterable, prefix='', suffix='', decimals=1, length=30, fill='█', print_end='\r'):
    total = len(iterable)

    def printProgressBar(iteration):
        formatter = '{0:.'+str(decimals)+'f}'
        percent = formatter.format(100 * (iteration / float(total)))
        filledLength = int(length * iteration // total)
        bar = fill * filledLength + '-' * (length - filledLength)
        print(f'\r{prefix} |{bar}| {percent}% {suffix}', end=print_end)

    printProgressBar(0)
    for i, item in enumerate(iterable):
        yield item
        printProgressBar(i + 1)
    print()
