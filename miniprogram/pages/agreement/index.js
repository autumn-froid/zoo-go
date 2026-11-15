const {request} = require('../../utils/api')

Page({
    data: {
        loading: true,
        role: 'owner',
        agreement: null,
    },

    onLoad(options) {
        const role = options.role === 'sitter' ? 'sitter' : 'owner'
        this.setData({role})
        this.loadAgreement(role)
    },

    async loadAgreement(role) {
        try {
            const agreement = await request(`/v1/agreements/current?role=${role}`)
            this.setData({loading: false, agreement})
            wx.setNavigationBarTitle({title: agreement.title || '服务协议'})
        } catch (error) {
            this.setData({loading: false})
            wx.showToast({title: error.message, icon: 'none'})
        }
    },
})
