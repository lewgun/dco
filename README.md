# 算法说明:
1. filter中对数据进行了分段，排序，细节如下:

    0. 下文中的 pN=> prepareN, cN=> commitN
    
    1. 对于一个goroutine来说，它负责放入连续的prepare&commit对，对一个prepare & commit对
       来说 commit的token值一定大于对应的prepare的token值，即类似于p1,c1,p2,c2...，单个goroutine
       中保持单调递增的顺序
       
    2. 对于多个goroutine来说，因为并发，所以可能出现...c0,p1,p2,c2,c1...,这种乱序的情况，而即使存在这种
       乱序情况，则仍然可以保证c0 < min(p1,p2),基于此，则可以将数据分段，所以在filter中，将数据按此分成多个
       序列S1=>...c0; S2=>p1,p2,c2,c1..., 每个序列内部基本有序
    
    3. 因为第2步生成的序列基本有序，所以可以采用insertion-sort的方法，将其排序，并将结果值放入channel

2. merge中对前一步产生的多个输出channel进行归并，此处采取的是优先级队列算法，首先从每一个client的channel中
   取出各自第一条数据，构成起始的队列，然后每次从队列中出队最小commit token的data并输出,然后从此data所属channel
   中再出队下一个元素，放入队列中，再从队列中出队次小data,重复此出队、放入新值，直到队列为空
   
# TestCase说明:
此testcase基于原始main.go改造，实现原理如下:

    1. 从生成的commit token 构建一个slice，并排序
     
    2. 从collect中出队所有收集到的消息到一个slice
     
    3. 比较两个序列的长度,如果长度不匹配，则表示测试未通过
     
    4. 按元素比较两个序列相应索引位置的元素，如果不等，则表示未通过
     
    5. 重复1～4的步骤多次，如果某一次未通过，则整个未通过