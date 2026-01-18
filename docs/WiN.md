#Epic:实现一个基于eino的docker智能化agent管理工具
#Story:作为用户,我想要一个可以与agent智能体交流,就能实现对本机的docker进行统一的管理 检查 操作的命令行工具
## Feature 1: 接收指令 
- task1: 能从命令行接收用户的指令,结合上下文理解用户的需求
- task2: 根据用户需求选择执行不同的功能,并留给用户自行决断的空间
- task3: 可以根据持久化的记录,如状态监控记录,日志记录,来提供用户查询和分析的功能
## Feature 2: 执行操作
- task1: 调用Docker Engine提供的API,执行docker容器的操作,如启动 停止 重启 查看日志等
## Feature 3: 状态监控
- task1: 监控docker容器的状态并持久化记录
## Feature 4: 日志收集
- task1: 收集docker容器的日志并持久化记录