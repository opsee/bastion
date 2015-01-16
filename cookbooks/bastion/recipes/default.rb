

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

file "/opt/bastion/bin/bastion" do
	owner "bastion"
	group "bastion"
	mode '0755'
	action :create
end

runit_service "bastion" do
	default_logger true
end