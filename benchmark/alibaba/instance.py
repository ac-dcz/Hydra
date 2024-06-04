from json import load
import time

from alibabacloud_vpc20160428.client import Client as Vpc20160428Client
from alibabacloud_vpc20160428 import models as vpc_20160428_models
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
    INSTANCE_NAME = 'lightDAG'
    SECURITY_GROUP_NAME = 'lightDAG'
    VPC_NAME = 'lightDAG'

    def __init__(self, settings):
        assert isinstance(settings, Settings)
        self.settings = settings
        with open(self.settings.accesskey_path,"r") as f:
            data = load(f)
        self.access_key_id = data["AccessKey ID"]
        self.access_key_secret = data["AccessKey Secret"]
        self.ecs_clients = OrderedDict()
        self.vpc_clients = OrderedDict()
        self.securities = OrderedDict()
        #为每个地区创建一个Client
        for region in settings.aws_regions:
            config = open_api_models.Config()
            config.access_key_id = self.access_key_id
            config.access_key_secret = self.access_key_secret
            config.region_id = region
            self.ecs_clients[region] = Ecs20140526Client(config)
            self.vpc_clients[region] = Vpc20160428Client(config)
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

        try:

            ids, ips = defaultdict(list), defaultdict(list)
            for region, client in self.ecs_clients.items():
                describe_instances_request = ecs_20140526_models.DescribeInstancesRequest(
                    region_id=region,
                    instance_type=self.settings.instance_type,
                    instance_name = self.INSTANCE_NAME,
                    internet_charge_type = 'PayByTraffic',
                    instance_charge_type = 'PostPaid',
                )

                resp = client.describe_instances_with_options(describe_instances_request, self.aliyun_runtime).to_map()
                for instance in resp['body']['Instances']['Instance']:
                    if instance['Status'] in state:
                        ids[region] += [instance['InstanceId']]
                        for ip in instance['PublicIpAddress']['IpAddress']:
                            ips[region] += [ip]

        except Exception as error:
            # 此处仅做打印展示，请谨慎对待异常处理，在工程项目中切勿直接忽略异常。
            # 错误 message
            print(error.message)
            # 诊断地址
            print(error.data.get("Recommend"))
            UtilClient.assert_as_string(error.message)

        return ids, ips

    def _wait(self, state):
        # Possible states are: 'pending', 'running', 'shutting-down',
        # 'terminated', 'stopping', and 'stopped'.
        while True:
            sleep(1)
            ids, _ = self._get(state)
            if sum(len(x) for x in ids.values()) == 0:
                break

    def _create_security_group(self, client , region):

        try:
            temp = {}
            # step 0: 查询vpc
            describe_vpcs_request = vpc_20160428_models.DescribeVpcsRequest(
                region_id = region,
                vpc_name='lightDAG'
            )

            resp = self.vpc_clients[region].describe_vpcs_with_options(describe_vpcs_request, self.aliyun_runtime).to_map()
            temp["VSwitchId"] = resp['body']['Vpcs']['Vpc'][0]['VSwitchIds']['VSwitchId'][0]
            temp['VpcId'] = resp['body']['Vpcs']['Vpc'][0]['VpcId']

            # step 1: 创建安全组
            create_security_group_request = ecs_20140526_models.CreateSecurityGroupRequest(
                region_id=region,
                description=self.INSTANCE_NAME,
                security_group_name=self.SECURITY_GROUP_NAME,
                vpc_id = temp['VpcId']
            )

            resp = client.create_security_group_with_options(create_security_group_request, self.aliyun_runtime).to_map()
            securityID = resp['body']['SecurityGroupId']
            temp['securityID'] = securityID
            self.securities[region] = temp
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
                        port_range= f'{self.settings.consensus_port}/{self.settings.consensus_port+4}',
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
            region_id = region,
            status = 'Available',
            image_owner_alias = 'system',
            instance_type = self.settings.instance_type,
            ostype = 'linux',
            architecture = 'x86_64',
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
        for region,client in self.ecs_clients.items():
            try:
                self._create_security_group(client,region)
            except Exception as e:
                raise BenchError('Failed to create security group', e)

        try:
            # Create all instances.
            size = instances * len(self.ecs_clients)
            progress = progress_bar(
                self.ecs_clients.items(), prefix=f'Creating {size} instances'
            )
            for region,client in progress:
                
                system_disk = ecs_20140526_models.RunInstancesRequestSystemDisk(
                    category='cloud_essd'
                )

                run_instances_request = ecs_20140526_models.RunInstancesRequest(
                    region_id = region,
                    image_id = self._get_ami(client,region),
                    instance_type = self.settings.instance_type,
                    instance_name = self.INSTANCE_NAME,
                    host_name = 'ubuntu',
                    internet_max_bandwidth_in = 100,
                    internet_max_bandwidth_out = 100,
                    unique_suffix = False,
                    internet_charge_type = 'PayByTraffic',
                    key_pair_name = self.settings.key_name,
                    system_disk=system_disk,
                    amount = instances,
                    min_amount = instances,
                    instance_charge_type = 'PostPaid',
                    security_group_id = self.securities[region]['securityID'],
                    v_switch_id = self.securities[region]['VSwitchId'],
                )

                client.run_instances_with_options(run_instances_request, self.aliyun_runtime)

            # Wait for the instances to boot.
            Print.info('Waiting for all instances to boot...')
            self._wait(['Pending'])
            Print.heading(f'Successfully created {size} new instances')
        except Exception as error:
            # 此处仅做打印展示，请谨慎对待异常处理，在工程项目中切勿直接忽略异常。
            # 错误 message
            print(error.message)
            # 诊断地址
            print(error.data.get("Recommend"))
            UtilClient.assert_as_string(error.message)

    def terminate_instances(self):
        
        try:
            ids, _ = self._get(['Pending', 'Running', 'Stopping', 'Stopped'])
            size = sum(len(x) for x in ids.values())
            if size != 0:
                # Terminate instances.
                for region, client in self.ecs_clients.items():
                    if ids[region]:
                        delete_instances_request = ecs_20140526_models.DeleteInstancesRequest(
                            region_id=region,
                            instance_id= ids[region],
                            force=True
                        )
                        client.delete_instances_with_options(delete_instances_request, self.aliyun_runtime)

                # Wait for all instances to properly shut down.
                Print.info('Waiting for all instances to shut down...')
                self._wait(['Pending', 'Running', 'Stopping', 'Stopped'])
            Print.heading(f'All instances are shut down')
            # Print.heading(f'Testbed of {size} instances destroyed')
        except Exception as e:
            raise BenchError('Failed to terminate instances', e)

    def delete_security(self):
        # step 2: 删除安全组
        for region,client in self.ecs_clients.items():
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

    def start_instances(self, max):
        size = 0
        try:
            ids, _ = self._get(['Stopping', 'Stopped'])
            for region, client in self.ecs_clients.items():
                for id in ids[region]:
                    target = ids[region]
                    target = target if len(target) < max else target[:max]
                    size += len(target)
                    start_instances_request = ecs_20140526_models.StartInstancesRequest(
                        region_id=region,
                        instance_id=target
                    )
                    client.start_instances_with_options(start_instances_request, self.aliyun_runtime)
            Print.heading(f'Starting {size} instances')

        except Exception as error:
            # 此处仅做打印展示，请谨慎对待异常处理，在工程项目中切勿直接忽略异常。
            # 错误 message
            print(error.message)
            # 诊断地址
            print(error.data.get("Recommend"))
            UtilClient.assert_as_string(error.message)

    def stop_instances(self):
        try:
            ids, _ = self._get(['Pending', 'Running'])
            for region, client in self.ecs_clients.items():
                if ids[region]:
                    stop_instances_request = ecs_20140526_models.StopInstancesRequest(
                        region_id=region,
                        instance_id=ids[region]
                    )
                    client.stop_instances_with_options(stop_instances_request, self.aliyun_runtime)
            size = sum(len(x) for x in ids.values())
            Print.heading(f'Stopping {size} instances')
        except Exception as error:
            # 错误 message
            print(error.message)
            # 诊断地址
            print(error.data.get("Recommend"))
            UtilClient.assert_as_string(error.message)

    def hosts(self, flat=False):

        _, ips = self._get(['Pending', 'Running'])
        return [x for y in ips.values() for x in y] if flat else ips

    def print_info(self):
        hosts = self.hosts()
        key = self.settings.key_path
        text = ''
        for region, ips in hosts.items():
            text += f'\n Region: {region.upper()}\n'
            for i, ip in enumerate(ips):
                new_line = '\n' if (i+1) % 6 == 0 else ''
                text += f'{new_line} {i}\tssh -i {key} root@{ip}\n'
        print(
            '\n'
            '----------------------------------------------------------------\n'
            ' INFO:\n'
            '----------------------------------------------------------------\n'
            f' Available machines: {sum(len(x) for x in hosts.values())}\n'
            f'{text}'
            '----------------------------------------------------------------\n'
        )
