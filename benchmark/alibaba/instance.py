from json import load
from alibabacloud_ecs20140526.client import Client as Ecs20140526Client
from alibabacloud_tea_openapi import models as open_api_models
from alibabacloud_ecs20140526 import models as ecs_20140526_models
from alibabacloud_tea_util import models as util_models
from alibabacloud_tea_util.client import Client as UtilClient

from collections import defaultdict, OrderedDict
from time import sleep

from benchmark.utils import Print, BenchError, progress_bar
from alibaba.settings import Settings, SettingsError



class InstanceManager:
    INSTANCE_NAME = 'lightDAG-node'
    SECURITY_GROUP_NAME = 'lightDAG'

    def __init__(self, settings):
        assert isinstance(settings, Settings)
        self.settings = settings
        with open(self.settings.accesskey_path,"r") as f:
            data = load(f)
        self.access_key_id = data["AccessKey ID"]
        self.access_key_secret = data["AccessKey Secret"]
        self.clients = OrderedDict()
        self.securities = OrderedDict()
        #为每个地区创建一个Client
        for region in settings.aws_regions:
            config = open_api_models.Config()
            config.access_key_id = self.access_key_id
            config.access_key_secret = self.access_key_secret
            config.region_id = region
            self.clients[region] = Ecs20140526Client(config)
        self.aliyun_runtime = util_models.RuntimeOptions()

    @classmethod
    def make(cls, settings_file='settings.json'):
        try:
            return cls(Settings.load(settings_file))
        except SettingsError as e:
            raise BenchError('Failed to load settings', e)

    def _get(self, state):
        # Possible states are: 'pending', 'running', 'shutting-down',
        # 'terminated', 'stopping', and 'stopped'.
        ids, ips = defaultdict(list), defaultdict(list)
        for region, client in self.clients.items():
            r = client.describe_instances(
                Filters=[
                    {
                        'Name': 'tag:Name',
                        'Values': [self.INSTANCE_NAME]
                    },
                    {
                        'Name': 'instance-state-name',
                        'Values': state
                    }
                ]
            )
            instances = [y for x in r['Reservations'] for y in x['Instances']]
            for x in instances:
                ids[region] += [x['InstanceId']]
                if 'PublicIpAddress' in x:
                    ips[region] += [x['PublicIpAddress']]
        return ids, ips

    def _wait(self, state):
        # Possible states are: 'pending', 'running', 'shutting-down',
        # 'terminated', 'stopping', and 'stopped'.
        while True:
            sleep(1)
            ids, _ = self._get(state)
            if sum(len(x) for x in ids.values()) == 0:
                break

    def _create_security_group(self, client,region):

        try:
            # step 1: 创建安全组
            create_security_group_request = ecs_20140526_models.CreateSecurityGroupRequest(
                region_id=region,
                description=self.INSTANCE_NAME,
                security_group_name=self.SECURITY_GROUP_NAME,
            )

            resp = client.create_security_group_with_options(create_security_group_request, self.aliyun_runtime).to_map()
            securityID = resp['body']['SecurityGroupId']
            self.securities[region] = securityID

            # step 2: 设置开放端口
            authorize_security_group_request = ecs_20140526_models.AuthorizeSecurityGroupRequest(
                region_id=region,
                security_group_id=securityID,
                permissions=[
                    ecs_20140526_models.AuthorizeSecurityGroupRequestPermissions(
                        priority='1',
                        ip_protocol='TCP',
                        source_cidr_ip='0.0.0.0/0',
                        port_range='22/22',
                        description='Debug SSH access'
                    ),
                    ecs_20140526_models.AuthorizeSecurityGroupRequestPermissions(
                        priority='1',
                        ip_protocol='TCP',
                        source_cidr_ip='0.0.0.0/0',
                        port_range= f'{self.settings.consensus_port}/{self.settings.consensus_port}',
                        description='Consensus port'
                    ),
                ]
            )
            client.authorize_security_group_with_options(authorize_security_group_request, self.aliyun_runtime)

        except Exception as error:
            # 此处仅做打印展示，请谨慎对待异常处理，在工程项目中切勿直接忽略异常。
            # 错误 message
            print(error.message)
            # 诊断地址
            print(error.data.get("Recommend"))
            UtilClient.assert_as_string(error.message)

    def _get_ami(self, client,region):
        # The AMI changes with regions.

        describe_images_request = ecs_20140526_models.DescribeImagesRequest(
            region_id='us-east-1',
            status='Available',
            image_owner_alias='system',
            instance_type='ecs.g6e.xlarge',
            ostype='linux',
            architecture='x86_64',
            filter=[
                ecs_20140526_models.DescribeImagesRequestFilter(
                    key='description',
                    value='Canonical, Ubuntu, 20.04 LTS, amd64 focal image build on 2020-10-26'
                )
            ],
            page_size=1,
            page_number=1
        )
        
        try:
            # 复制代码运行请自行打印 API 的返回值
            resp = client.describe_images_with_options(describe_images_request, self.aliyun_runtime).to_map()
            return resp['body']['Images']['Image'][0]['ImageId']

        except Exception as error:
            # 此处仅做打印展示，请谨慎对待异常处理，在工程项目中切勿直接忽略异常。
            # 错误 message
            print(error.message)
            # 诊断地址
            print(error.data.get("Recommend"))
            UtilClient.assert_as_string(error.message)

    def create_instances(self, instances):
        assert isinstance(instances, int) and instances > 0

        # Create the security group in every region.
        for region,client in self.clients.items():
            try:
                self._create_security_group(client,region)
            except Exception as e:
                raise BenchError('Failed to create security group', e)

        try:
            # Create all instances.
            size = instances * len(self.clients)
            progress = progress_bar(
                self.clients.items(), prefix=f'Creating {size} instances'
            )
            for region,client in progress:
                client.run_instances(
                    ImageId=self._get_ami(client,region),
                    InstanceType=self.settings.instance_type,
                    KeyName=self.settings.key_name,
                    MaxCount=instances,
                    MinCount=instances,
                    SecurityGroups=[self.SECURITY_GROUP_NAME],
                    TagSpecifications=[{
                        'ResourceType': 'instance',
                        'Tags': [{
                            'Key': 'Name',
                            'Value': self.INSTANCE_NAME
                        }]
                    }],
                    EbsOptimized=True,
                    BlockDeviceMappings=[{
                        'DeviceName': '/dev/sda1',
                        'Ebs': {
                            'VolumeType': 'gp2',
                            'VolumeSize': 200,
                            'DeleteOnTermination': True
                        }
                    }],
                )

            # Wait for the instances to boot.
            Print.info('Waiting for all instances to boot...')
            self._wait(['pending'])
            Print.heading(f'Successfully created {size} new instances')
        except ClientError as e:
            raise BenchError('Failed to create AWS instances', AWSError(e))

    def terminate_instances(self):
        
        try:
            # ids, _ = self._get(['pending', 'running', 'stopping', 'stopped'])
            # size = sum(len(x) for x in ids.values())
            # if size == 0:
            #     Print.heading(f'All instances are shut down')
            #     return

            # # Terminate instances.
            # for region, client in self.clients.items():
            #     if ids[region]:
            #         client.terminate_instances(InstanceIds=ids[region])

            # # Wait for all instances to properly shut down.
            # Print.info('Waiting for all instances to shut down...')
            # self._wait(['shutting-down'])
            for region,client in self.clients.items():
                describe_security_groups_request = ecs_20140526_models.DescribeSecurityGroupsRequest(
                    region_id=region,
                    security_group_name=self.SECURITY_GROUP_NAME,
                )
                resp = client.describe_security_groups_with_options(describe_security_groups_request, self.aliyun_runtime).to_map()
                for group in resp['body']["SecurityGroups"]["SecurityGroup"]:
                    delete_security_group_request = ecs_20140526_models.DeleteSecurityGroupRequest(
                        region_id=region,
                        security_group_id=group["SecurityGroupId"],
                    )
                    client.delete_security_group_with_options(delete_security_group_request, self.aliyun_runtime)

            # Print.heading(f'Testbed of {size} instances destroyed')
        except Exception as e:
            raise BenchError('Failed to terminate instances', e)

    def start_instances(self, max):
        size = 0
        try:
            ids, _ = self._get(['stopping', 'stopped'])
            for region, client in self.clients.items():
                if ids[region]:
                    target = ids[region]
                    target = target if len(target) < max else target[:max]
                    size += len(target)
                    client.start_instances(InstanceIds=target)
            Print.heading(f'Starting {size} instances')
        except ClientError as e:
            raise BenchError('Failed to start instances', AWSError(e))

    def stop_instances(self):
        try:
            ids, _ = self._get(['pending', 'running'])
            for region, client in self.clients.items():
                if ids[region]:
                    client.stop_instances(InstanceIds=ids[region])
            size = sum(len(x) for x in ids.values())
            Print.heading(f'Stopping {size} instances')
        except ClientError as e:
            raise BenchError(AWSError(e))

    def hosts(self, flat=False):
        try:
            _, ips = self._get(['pending', 'running'])
            return [x for y in ips.values() for x in y] if flat else ips
        except ClientError as e:
            raise BenchError('Failed to gather instances IPs', AWSError(e))

    def print_info(self):
        hosts = self.hosts()
        key = self.settings.key_path
        text = ''
        for region, ips in hosts.items():
            text += f'\n Region: {region.upper()}\n'
            for i, ip in enumerate(ips):
                new_line = '\n' if (i+1) % 6 == 0 else ''
                text += f'{new_line} {i}\tssh -i {key} ubuntu@{ip}\n'
        print(
            '\n'
            '----------------------------------------------------------------\n'
            ' INFO:\n'
            '----------------------------------------------------------------\n'
            f' Available machines: {sum(len(x) for x in hosts.values())}\n'
            f'{text}'
            '----------------------------------------------------------------\n'
        )
