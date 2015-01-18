

user "bastion" do
	shell "/bin/bash"
	home "/opt/bastion"
	system true
end

%w[ bin etc srv ].each do |dir|
	directory "/opt/bastion/#{dir}" do
		owner "bastion"
		group "bastion"
		mode '0755'
		recursive true
	end
end

cookbook_file "bastion" do
	path "/opt/bastion/bin/bastion"
	owner "bastion"
	group "bastion"
	mode '0755'
	action :create
end

cookbook_file "demo_data.json" do
	path "/opt/bastion/etc/demo_data.json"
	owner "bastion"
	group "bastion"
	mode '0755'
	action :create
end

runit_service "bastion" do
	default_logger true
end